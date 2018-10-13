package main

import (
	"fmt"
	"github.com/nsf/termbox-go"
	"github.com/urfave/cli"
	"math"
	"os"
	"os/exec"
	"strings"
)

func main() {

	app := cli.NewApp()
	app.Name = "combo"
	app.HelpName = os.Args[0]
	app.Usage = "terminal combo widget"
	app.Version = "v1.0.0"
	app.Writer = os.Stderr
	app.ErrWriter = app.Writer

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "callback, c",
			Usage: "callback the command with the last selected item as argument after the selection",
		},
		cli.BoolFlag{
			Name:  "append, a",
			Usage: "callback the command with all the last selected items as arguments (valid only when --callback is enabled)",
		},
		cli.StringFlag{
			Name:  "separator, s",
			Usage: "separator to use when appending selected items as arguments (valid only when --append is enabled)",
		},
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "force a selection, does not allow free text",
		},
	}

	app.Action = mainAction

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

}

func mainAction(c *cli.Context) error {

	if !c.Bool("c") && c.Bool("a") {
		return fmt.Errorf("--append is valid only if --callback is enabled")
	}

	if !c.Bool("a") && c.String("s") != "" {
		return fmt.Errorf("--separator is valid only if --append is enabled")
	}

	if !c.Args().Present() {
		return fmt.Errorf("a command is required")
	}

	out, err := pickUI(c)
	if err != nil {
		return err
	}

	if out == "" {
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "%s", out)
	os.Exit(0)

	return nil
}

func pickUI(c *cli.Context) (string, error) {

	err := termbox.Init()
	if err != nil {
		return "", err
	}
	defer func() {
		termbox.Close()
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "panic: %v\n", err)
		}
	}()

	termbox.SetInputMode(termbox.InputEsc)
	termbox.SetOutputMode(termbox.OutputNormal)

	return pickLoop(c)
}

func pickLoop(c *cli.Context) (string, error) {

	lastOut := ""

	var cmdBase = c.Args()
	var cmdArgs []string

	for {

		var cmd []string
		cmd = append(cmd, cmdBase...)
		if len(cmdArgs) != 0 {
			if c.String("s") != "" {
				cmd = append(cmd, strings.Join(cmdArgs, c.String("s")))
			} else {
				cmd = append(cmd, cmdArgs...)
			}
		}

		items, err := run(cmd)
		if err != nil {
			return "", err
		}
		if len(items) == 0 && lastOut != "" {
			return lastOut, nil
		}

		out, err := pick(c, cmd, items)
		if err != nil {
			return "", err
		}
		if out == "" {
			return "", err
		}

		if !c.Bool("c") {
			return out, nil
		}

		lastOut = out

		if !c.Bool("a") {
			cmdArgs = nil
		}
		cmdArgs = append(cmdArgs, out)
	}
}

func pick(c *cli.Context, cmd, allItems []string) (string, error) {

	input := ""
	sel := -1

	for {

		err := termbox.Clear(termbox.ColorBlack, termbox.ColorBlack)
		if err != nil {
			return "", err
		}

		termw, termh := termbox.Size()

		pageSize := termh - 2

		printf(0, 0, termbox.ColorWhite, termbox.ColorBlack, "$ %*s", -termw, strings.Join(cmd, " "))

		prompt := fmt.Sprintf("> %s", input)
		printf(0, 1, termbox.ColorBlack, termbox.ColorGreen, "%*s", -termw, prompt)
		termbox.SetCursor(len(prompt), 1)

		items := filter(allItems, input)

		if len(items) == 0 {

			sel = -1

		} else {

			if sel < 0 {
				sel = 0
			} else if sel >= len(items) {
				sel = len(items) - 1
			}

			pageStart := int(math.Floor(float64(sel)/float64(pageSize))) * pageSize
			pageEnd := pageStart + pageSize
			if pageEnd > len(items) {
				pageEnd = len(items)
			}

			for idx, item := range items[pageStart:pageEnd] {
				fg := termbox.ColorWhite
				bg := termbox.ColorBlack
				if idx == sel%pageSize {
					fg = termbox.ColorBlack
					bg = termbox.ColorBlue
				}
				printf(0, 2+idx, fg, bg, "%*s", -termw, item)
			}
		}

		err = termbox.Flush()
		if err != nil {
			return "", err
		}

		ev := termbox.PollEvent()

		if ev.Type == termbox.EventKey {

			if ev.Ch == 0 {

				if ev.Key == termbox.KeyEsc {

					return "", nil

				} else if ev.Key == termbox.KeyEnter {

					if sel >= 0 {
						return items[sel], nil
					}

					if !c.Bool("f") {
						return input, nil
					}

				} else if ev.Key == termbox.KeySpace {

					input += " "

				} else if ev.Key == termbox.KeyArrowUp {

					sel--

				} else if ev.Key == termbox.KeyArrowDown {

					sel++

				} else if ev.Key == termbox.KeyPgup {

					sel -= pageSize

				} else if ev.Key == termbox.KeyPgdn {

					sel += pageSize

				} else if ev.Key == termbox.KeyBackspace || ev.Key == termbox.KeyBackspace2 {

					if len(input) > 0 {
						input = input[:len(input)-1]
					}
				}

				continue
			}

			input += string(ev.Ch)
		}
	}
}

func filter(items []string, filter string) []string {

	if filter == "" {
		return items
	}

	var filtered []string

	terms := strings.Split(filter, " ")

items:
	for _, item := range items {

		for _, term := range terms {
			if term != "" && !strings.Contains(item, term) {
				continue items
			}
		}

		filtered = append(filtered, item)
	}

	return filtered
}

func run(cmd []string) ([]string, error) {

	if len(cmd) == 0 {
		return nil, fmt.Errorf("no command to run")
	}

	var name = cmd[0]
	var args []string
	if len(cmd) > 1 {
		args = cmd[1:]
	}

	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("%s %v: %v", name, args, err)
	}

	var lines []string

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines, nil
}

func printf(x, y int, fg, bg termbox.Attribute, format string, args ...interface{}) {

	msg := fmt.Sprintf(format, args...)

	for _, c := range msg {
		termbox.SetCell(x, y, c, fg, bg)
		x++
	}
}
