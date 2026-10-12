package cli_test

import (
	"bytes"
	"errors"
	"hm1/internal/cli"
	"hm1/internal/service"
	"hm1/internal/storage"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSplit(t *testing.T) {
	for _, tc := range []struct {
		line string
		want []string
	}{
		{`add --title "Задача с пробелами"`, []string{"add", "--title", "Задача с пробелами"}},
		{`edit 1 --title ""`, []string{"edit", "1", "--title", ""}},
		{`import "C:\Users\My Documents\tasks.json"`, []string{"import", `C:\Users\My Documents\tasks.json`}},
		{`add --title 'Текст "в кавычках"'`, []string{"add", "--title", `Текст "в кавычках"`}},
		{`add --title "Текст \"в кавычках\""`, []string{"add", "--title", `Текст "в кавычках"`}},
	} {
		got, err := cli.Split(tc.line)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%q: %v %v", tc.line, got, err)
		}
	}
	if _, err := cli.Split(`add "unfinished`); err == nil {
		t.Fatal("unclosed quote accepted")
	}
}

func TestInteractiveCommandsRecoverFromErrors(t *testing.T) {
	repo, err := storage.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	s := service.New(repo, func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) })
	input := `help
add --title "Первая задача" --deadline 2030-01-02
edit 1 --title "Новое имя"
status 1 in_progress
list --search "новое"
get 999
edit 1 --title ""
status 1 done
edit 1 --deadline 2030-01-03
list --page 0
delete 1
add --title "Следующая" --deadline 2030-01-04
get 2
exit
add --title "После выхода" --deadline 2030-01-04
`
	var out bytes.Buffer
	if err := cli.New(s, strings.NewReader(input), &out).Run(); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Команды", "Всего задач: 1", "задача не найдена", "заголовок не может быть пустым", "нельзя изменять", "page и limit", `2 | 2030-01-04 | planned | "Следующая"`} {
		if !strings.Contains(out.String(), text) {
			t.Fatalf("missing %q in %s", text, out.String())
		}
	}
	r, err := s.List(service.Filter{})
	if err != nil || r.Total != 1 {
		t.Fatalf("exit didn't stop: %+v %v", r, err)
	}
}

type badReader struct{ err error }

func (b badReader) Read([]byte) (int, error) { return 0, b.err }
func TestInputErrorPropagates(t *testing.T) {
	cause := errors.New("input failed")
	if err := cli.New(nil, badReader{cause}, &bytes.Buffer{}).Run(); !errors.Is(err, cause) {
		t.Fatal(err)
	}
}
