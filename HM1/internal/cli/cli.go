package cli

import (
	"bufio"
	"flag"
	"fmt"
	"hm1/internal/domain"
	"hm1/internal/service"
	"io"
	"os"
	"strconv"
)

const help = `Команды (дедлайн — YYYY-MM-DD):
  help
  add --title "Заголовок задачи" --deadline 2030-12-31
  list [--status planned|in_progress|canceled|done] [--overdue] [--search "текст"] [--page 1] [--limit 20]
  get ID
  status ID planned|in_progress|canceled|done
  edit ID [--title "Новый заголовок"] [--deadline 2030-12-31]
  delete ID
  import "путь к файлу.json"
  exit
Заголовки и пути с пробелами заключайте в кавычки.
`

type App struct {
	service *service.Service
	in      io.Reader
	out     io.Writer
}

func New(s *service.Service, in io.Reader, out io.Writer) *App {
	return &App{service: s, in: in, out: out}
}

func (a *App) Run() error {
	if _, err := fmt.Fprintln(a.out, "Таск-трекер. Введите help для списка команд."); err != nil {
		return fmt.Errorf("вывести приветствие: %w", err)
	}
	scanner := bufio.NewScanner(a.in)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for {
		if _, err := fmt.Fprint(a.out, "> "); err != nil {
			return fmt.Errorf("вывести приглашение: %w", err)
		}
		if !scanner.Scan() {
			break
		}
		args, err := Split(scanner.Text())
		if err == nil && len(args) == 0 {
			continue
		}
		if err == nil && args[0] == "exit" && len(args) == 1 {
			return nil
		}
		var output string
		if err == nil {
			output, err = a.execute(args)
		}
		if err != nil {
			output = fmt.Sprintf("Ошибка: %v\n", err)
		}
		if _, err := io.WriteString(a.out, output); err != nil {
			return fmt.Errorf("вывести результат: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("прочитать команду: %w", err)
	}
	return nil
}

func taskText(t domain.Task) string {
	return fmt.Sprintf("%d | %s | %s | %q\n", t.ID, t.Deadline.Format("2006-01-02"), t.Status, t.Title)
}

func parseID(value string) (domain.ID, error) {
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("прочитать ID: %w", err)
	}
	if n == 0 {
		return 0, fmt.Errorf("ID должен быть положительным: %w", domain.ErrInvalid)
	}
	return domain.ID(n), nil
}

func flags(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	return f
}

func parseFlags(f *flag.FlagSet, args []string) error {
	if err := f.Parse(args); err != nil {
		return fmt.Errorf("параметры команды: %w", err)
	}
	if f.NArg() != 0 {
		return fmt.Errorf("лишние аргументы; см. help: %w", domain.ErrInvalid)
	}
	return nil
}

func (a *App) execute(args []string) (string, error) {
	switch args[0] {
	case "help":
		if len(args) == 1 {
			return help, nil
		}
	case "add":
		f := flags("add")
		title := f.String("title", "", "")
		date := f.String("deadline", "", "")
		if err := parseFlags(f, args[1:]); err != nil {
			return "", err
		}
		deadline, err := domain.ParseDeadline(*date)
		if err != nil {
			return "", err
		}
		t, err := a.service.Add(*title, deadline)
		if err != nil {
			return "", err
		}
		return "Добавлена задача:\n" + taskText(t), nil
	case "list":
		f := flags("list")
		status := f.String("status", "", "")
		overdue := f.Bool("overdue", false, "")
		search := f.String("search", "", "")
		page := f.Int("page", 1, "")
		limit := f.Int("limit", 20, "")
		if err := parseFlags(f, args[1:]); err != nil {
			return "", err
		}
		if *page < 1 || *limit < 1 {
			return "", fmt.Errorf("page и limit должны быть положительными: %w", domain.ErrInvalid)
		}
		r, err := a.service.List(service.Filter{Status: domain.Status(*status), Overdue: *overdue, Search: *search, Page: *page, Limit: *limit})
		if err != nil {
			return "", err
		}
		out := fmt.Sprintf("Всего задач: %d. Всего страниц: %d. Запрошенная страница: %d.\n", r.Total, r.Pages, r.Page)
		for _, t := range r.Tasks {
			out += taskText(t)
		}
		if len(r.Tasks) == 0 {
			out += "На этой странице задач нет.\n"
		}
		return out, nil
	case "get", "delete", "status", "edit":
		if len(args) < 2 {
			break
		}
		id, err := parseID(args[1])
		if err != nil {
			return "", err
		}
		switch args[0] {
		case "get":
			if len(args) != 2 {
				break
			}
			t, err := a.service.Get(id)
			if err != nil {
				return "", err
			}
			return taskText(t), nil
		case "delete":
			if len(args) != 2 {
				break
			}
			if err := a.service.Delete(id); err != nil {
				return "", err
			}
			return "Задача удалена.\n", nil
		case "status":
			if len(args) != 3 {
				break
			}
			if err := a.service.ChangeStatus(id, domain.Status(args[2])); err != nil {
				return "", err
			}
			return "Статус изменён.\n", nil
		case "edit":
			f := flags("edit")
			title := f.String("title", "", "")
			date := f.String("deadline", "", "")
			if err := parseFlags(f, args[2:]); err != nil {
				return "", err
			}
			visited := map[string]bool{}
			f.Visit(func(v *flag.Flag) { visited[v.Name] = true })
			change := service.Edit{}
			if visited["title"] {
				change.Title = title
			}
			if visited["deadline"] {
				d, err := domain.ParseDeadline(*date)
				if err != nil {
					return "", err
				}
				change.Deadline = &d
			}
			if err := a.service.Edit(id, change); err != nil {
				return "", err
			}
			return "Задача изменена.\n", nil
		}
	case "import":
		if len(args) != 2 {
			break
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			return "", fmt.Errorf("прочитать файл импорта: %w", err)
		}
		r, err := a.service.ImportJSON(data)
		if err != nil {
			return "", err
		}
		out := fmt.Sprintf("Добавлено задач: %d. Ошибочных записей: %d.\n", r.Added, len(r.Issues))
		for _, issue := range r.Issues {
			out += fmt.Sprintf("Ошибка: %v\n", issue.Err)
		}
		return out, nil
	}
	return "", fmt.Errorf("неизвестная команда или неверные аргументы; см. help: %w", domain.ErrInvalid)
}
