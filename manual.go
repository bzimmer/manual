package manual

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/urfave/cli/v2"
)

//go:embed templates/_commands.md
var content embed.FS //nolint: gochecknoglobals

func envvars(flags []cli.Flag) []string {
	vars := make(map[string]bool)
	for _, flag := range flags {
		switch v := flag.(type) {
		case *cli.StringFlag:
			for _, env := range v.EnvVars {
				vars[env] = true
			}
		case *cli.BoolFlag:
			for _, env := range v.EnvVars {
				vars[env] = true
			}
		case *cli.Int64Flag:
			for _, env := range v.EnvVars {
				vars[env] = true
			}
		}
	}
	var k []string
	for v := range vars {
		k = append(k, v)
	}
	sort.Strings(k)
	return k
}

type command struct {
	Cmd     *cli.Command
	Lineage []*cli.Command
}

func (c *command) String() string { return c.fullname(" ") }

func (c *command) aliases() []string {
	if c.Cmd.Aliases == nil {
		return nil
	}
	var s []string
	for i := range c.Cmd.Aliases {
		if c.Cmd.Aliases[i] != "" {
			s = append(s, c.Cmd.Aliases[i])
		}
	}
	return s
}

func (c *command) fullname(sep string) string {
	var names []string
	for i := range c.Lineage {
		names = append(names, c.Lineage[i].Name)
	}
	names = append(names, c.Cmd.Name)
	return strings.Join(names, sep)
}

func read(name string, paths []string) (string, error) {
	var contents []byte
	for i := range paths {
		fn := filepath.Join(paths[i], name)
		fp, err := os.Open(fn)
		if err != nil {
			if !os.IsNotExist(err) {
				return "", err
			}
			continue
		}
		contents, err = io.ReadAll(fp)
		if err != nil {
			return "", err
		}
	}
	if contents == nil {
		// one last attempt at reading from the embedded content
		fp, err := content.Open(filepath.Join("templates", name))
		if err != nil {
			return "", err
		}
		contents, err = io.ReadAll(fp)
		if err != nil {
			return "", err
		}
	}
	return string(contents), nil
}

func commandsTemplate(paths []string) (*template.Template, error) {
	man, err := read("_commands.md", paths)
	if err != nil {
		return nil, err
	}
	return template.New("commands").
		Funcs(map[string]interface{}{
			"partial": func(fn string) (string, error) {
				usage, err := read(fn+".md", paths)
				if err != nil {
					if os.IsNotExist(err) {
						// ok to skip any commands without usage documentation
						return "", nil
					}
					return "", err
				}
				return usage, nil
			},
			"join": strings.Join,
			"fullname": func(c *command, sep string) string {
				return c.fullname(sep)
			},
			"aliases": func(c *command) []string {
				return c.aliases()
			},
			"names": func(f cli.Flag) string {
				// the first name is always the long name so skip it
				if len(f.Names()) <= 1 {
					return ""
				}
				return strings.Join(f.Names()[1:], ", ")
			},
			"envvars": func(f cli.Flag) string {
				return strings.Join(envvars([]cli.Flag{f}), ", ")
			},
			"description": func(f cli.Flag) string {
				if x, ok := f.(cli.DocGenerationFlag); ok {
					return x.GetUsage()
				}
				return ""
			},
		}).
		Parse(man)
}

func lineate(cmds, lineage []*cli.Command) []*command {
	var commands []*command
	for i := range cmds {
		if cmds[i].Hidden {
			continue
		}
		commands = append(commands, &command{Cmd: cmds[i], Lineage: lineage})
		commands = append(commands, lineate(cmds[i].Subcommands, append(lineage, cmds[i]))...)
	}
	sort.SliceStable(commands, func(i, j int) bool {
		return commands[i].fullname("") < commands[j].fullname("")
	})
	return commands
}

func Manual() *cli.Command {
	return &cli.Command{
		Name:    "manual",
		Usage:   "Generate the user manual",
		Aliases: []string{"man"},
		Hidden:  true,
		Action: func(c *cli.Context) error {
			var buffer bytes.Buffer
			commands := lineate(c.App.Commands, nil)
			t, err := commandsTemplate(c.Args().Slice())
			if err != nil {
				return err
			}
			if err := t.Execute(&buffer, map[string]interface{}{
				"Name":        c.App.Name,
				"Description": c.App.Description,
				"GlobalFlags": c.App.Flags,
				"Commands":    commands,
			}); err != nil {
				return err
			}
			fmt.Fprint(c.App.Writer, buffer.String())
			return nil
		},
	}
}

func Commands() *cli.Command {
	return &cli.Command{
		Name:  "commands",
		Usage: "Print all possible commands",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "description",
				Aliases: []string{"d"},
				Usage:   "Print the command description as a comment",
			},
			&cli.BoolFlag{
				Name:    "relative",
				Aliases: []string{"r"},
				Usage:   "Specify the command relative to the current working directory",
			},
		},
		Action: func(c *cli.Context) error {
			cmd := c.App.Name
			if c.Bool("relative") {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				cmd, err = os.Executable()
				if err != nil {
					return err
				}
				cmd, err = filepath.Rel(cwd, cmd)
				if err != nil {
					return err
				}
				cmd, err = filepath.Abs(cmd)
				if err != nil {
					return err
				}
			}
			desc := c.Bool("description")
			for _, m := range lineate(c.App.Commands, nil) {
				if m.Cmd.Action != nil {
					if desc {
						for _, x := range []string{m.Cmd.Description, m.Cmd.Usage} {
							if x != "" {
								fmt.Fprintln(c.App.Writer, "#", x)
								break
							}
						}
					}
					fmt.Fprintln(c.App.Writer, cmd+" "+m.fullname(" "))
				}
			}
			return nil
		},
	}
}

func EnvVars() *cli.Command {
	return &cli.Command{
		Name:        "envvars",
		Usage:       "Print all the possible environment variables",
		Description: "Useful for creating a .env file for all possible environment variables",
		Action: func(c *cli.Context) error {
			flags := c.App.Flags
			for _, cmd := range lineate(c.App.Commands, nil) {
				flags = append(flags, cmd.Cmd.Flags...)
			}
			vars := envvars(flags)
			for _, v := range vars {
				fmt.Fprintln(c.App.Writer, v+"=")
			}
			return nil
		},
	}
}
