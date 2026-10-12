package main

import (
	"flag"
	"fmt"
	"hm1/internal/cli"
	"hm1/internal/service"
	"hm1/internal/storage"
	"os"
	"time"
)

func main() {
	data := flag.String("data", "data/tasks.json", "путь к хранилищу задач")
	flag.Parse()
	repo, err := storage.NewFile(*data)
	if err == nil {
		err = cli.New(service.New(repo, time.Now), os.Stdin, os.Stdout).Run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}
}
