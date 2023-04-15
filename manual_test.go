package manual_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"

	"github.com/bzimmer/manual"
)

type Harness struct {
	Name, Err string
	Args      []string
	Before    cli.BeforeFunc
	After     cli.AfterFunc
}

func Run(t *testing.T, tt *Harness, cmd *cli.Command) {
	a := assert.New(t)
	app := NewTestApp(t, tt.Name, cmd)
	if tt.Before != nil {
		f := app.Before
		app.Before = func(c *cli.Context) error {
			if f != nil {
				if err := f(c); err != nil {
					return err
				}
			}
			return tt.Before(c)
		}
	}
	if tt.After != nil {
		f := app.After
		app.After = func(c *cli.Context) error {
			if f != nil {
				if err := f(c); err != nil {
					return err
				}
			}
			return tt.After(c)
		}
	}

	err := app.RunContext(context.Background(), tt.Args)
	switch tt.Err == "" {
	case true:
		a.NoError(err)
	case false:
		a.Error(err)
		a.Contains(err.Error(), tt.Err)
	}
}

func NewTestApp(t *testing.T, name string, cmd *cli.Command) *cli.App {
	return &cli.App{
		Name:     name,
		HelpName: name,
		ExitErrHandler: func(c *cli.Context, err error) {
			if err == nil {
				return
			}
			t.Error(err)
		},
		Commands: []*cli.Command{cmd},
	}
}

func TestManual(t *testing.T) {
	a := assert.New(t)

	dir := t.TempDir()

	tests := []*Harness{
		{
			Name: "manual",
			Args: []string{"demo", "manual", "templates/"},
			Before: func(c *cli.Context) error {
				c.App.Writer = &bytes.Buffer{}
				return nil
			},
			After: func(c *cli.Context) error {
				s := c.App.Writer.(*bytes.Buffer).String()
				a.Greater(len(s), 0)
				a.Contains(s, "manual")
				return nil
			},
		},
		{
			Name: "manual with output path",
			Args: []string{"demo", "manual", "-o", filepath.Join(dir, "output.md"), "templates/"},
			Before: func(c *cli.Context) error {
				c.App.Writer = &bytes.Buffer{}
				return nil
			},
			After: func(c *cli.Context) error {
				s := c.App.Writer.(*bytes.Buffer).String()
				a.Equal(len(s), 0)
				return nil
			},
		},
		{
			Name: "manual (not hidden)",
			Args: []string{"demo", "manual", "templates/"},
			Before: func(c *cli.Context) error {
				c.App.Writer = &bytes.Buffer{}
				a.Equal("manual", c.App.Commands[0].Name)
				c.App.Commands[0].Hidden = false
				return nil
			},
			After: func(c *cli.Context) error {
				s := c.App.Writer.(*bytes.Buffer).String()
				a.Greater(len(s), 0)
				a.Contains(s, "* [manual](#manual)")
				return nil
			},
		},
		{
			Name: "manual with embedded content",
			Args: []string{"demo", "manual"},
			Before: func(c *cli.Context) error {
				c.App.Writer = &bytes.Buffer{}
				a.Equal("manual", c.App.Commands[0].Name)
				c.App.Commands[0].Hidden = false
				return nil
			},
			After: func(c *cli.Context) error {
				s := c.App.Writer.(*bytes.Buffer).String()
				a.Greater(len(s), 0)
				a.Contains(s, "* [manual](#manual)")
				return nil
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			Run(t, tt, manual.Manual())
		})
	}
}

func TestCommands(t *testing.T) {
	a := assert.New(t)

	tests := []*Harness{
		{
			Name: "commands",
			Args: []string{"demo", "commands"},
			Before: func(c *cli.Context) error {
				c.App.Writer = bytes.NewBufferString("")
				return nil
			},
			After: func(c *cli.Context) error {
				s := c.App.Writer.(*bytes.Buffer).String()
				a.Contains(s, "commands commands")
				return nil
			},
		},
		{
			Name: "commands relative",
			Args: []string{"demo", "commands", "--relative"},
			Before: func(c *cli.Context) error {
				c.App.Writer = bytes.NewBufferString("")
				return nil
			},
			After: func(c *cli.Context) error {
				s := c.App.Writer.(*bytes.Buffer).String()
				a.Contains(s, "manual.test commands")
				return nil
			},
		},
		{
			Name: "commands descriptions",
			Args: []string{"demo", "commands", "--description"},
			Before: func(c *cli.Context) error {
				c.App.Writer = bytes.NewBufferString("")
				c.App.Commands = append(
					c.App.Commands,
					&cli.Command{
						Name:        "something",
						Description: "This is a description of `something`",
						Action: func(c *cli.Context) error {
							return nil
						},
					})
				return nil
			},
			After: func(c *cli.Context) error {
				s := c.App.Writer.(*bytes.Buffer).String()
				a.Contains(s, "# This is a description of `something`")
				return nil
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			Run(t, tt, manual.Commands())
		})
	}
}

func TestEnvVars(t *testing.T) {
	a := assert.New(t)

	cmds := []*cli.Command{
		{
			Name: "something",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "today",
					EnvVars: []string{"BARBAR"},
				},
				&cli.Int64Flag{
					Name:    "tomorrow",
					EnvVars: []string{"BAZBAZ"},
				},
			},
			Subcommands: []*cli.Command{
				{
					Name: "else",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name: "yesterday",
						},
						&cli.StringFlag{
							Name:    "fourscore",
							EnvVars: []string{"FOURSCORE"},
						},
					},
				},
			},
		},
	}
	tests := []*Harness{
		{
			Name: "envvars",
			Args: []string{"demo", "envvars"},
			Before: func(c *cli.Context) error {
				c.App.Writer = bytes.NewBufferString("")
				c.App.Flags = []cli.Flag{
					&cli.StringFlag{
						Name:    "foo",
						EnvVars: []string{"FOO"},
					},
				}
				c.App.Commands = append(
					c.App.Commands,
					cmds...)
				return nil
			},
			After: func(c *cli.Context) error {
				s := c.App.Writer.(*bytes.Buffer).String()
				a.Greater(len(s), 0)
				a.Contains(s, "FOO=")
				a.Contains(s, "BARBAR=")
				a.Contains(s, "BAZBAZ=")
				a.Contains(s, "FOURSCORE=")
				return nil
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			Run(t, tt, manual.EnvVars())
		})
	}
}
