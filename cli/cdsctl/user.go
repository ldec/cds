package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ovh/cds/cli"
)

var (
	userCmd = cli.Command{
		Name:  "user",
		Short: "Manage CDS user",
	}

	usr = cli.NewCommand(userCmd, nil,
		[]*cobra.Command{
			cli.NewListCommand(userListCmd, userListRun, nil),
			cli.NewGetCommand(userShowCmd, userShowRun, nil),
			cli.NewCommand(userResetCmd, userResetRun, nil),
			cli.NewCommand(userConfirmCmd, userConfirmRun, nil),
		})
)

var userListCmd = cli.Command{
	Name:  "list",
	Short: "List CDS users",
}

func userListRun(v cli.Values) (cli.ListResult, error) {
	users, err := client.UserList()
	if err != nil {
		return nil, err
	}
	return cli.AsListResult(users), nil
}

var userShowCmd = cli.Command{
	Name:  "show",
	Short: "Show CDS user details",
	Args: []cli.Arg{
		{Name: "username"},
	},
}

func userShowRun(v cli.Values) (interface{}, error) {
	u, err := client.UserGet(v["username"])
	if err != nil {
		return nil, err
	}
	return *u, nil
}

var userResetCmd = cli.Command{
	Name:  "reset",
	Short: "Reset CDS user password",
	OptionnalArgs: []cli.Arg{
		{Name: "username"},
		{Name: "email"},
	},
}

func userResetRun(v cli.Values) error {
	username := v["username"]
	if username == "" {
		username = cfg.User
	}
	if username == "" {
		fmt.Printf("Username: ")
		username = cli.ReadLine()
	} else {
		fmt.Println("Username:", username)
	}

	email := v["email"]
	if email == "" {
		fmt.Printf("Email: ")
		email = cli.ReadLine()
	} else {
		fmt.Println("Email:", email)
	}

	client.UserReset(cfg.User, email)
	return nil
}

var userConfirmCmd = cli.Command{
	Name:  "confirm",
	Short: "Confirm CDS user password reset",
}

func userConfirmRun(v cli.Values) error {
	return nil
}
