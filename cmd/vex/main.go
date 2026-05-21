package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jimbersoftware/vex/client"
	"github.com/jimbersoftware/vex/internal/version"
	"github.com/spf13/cobra"
)

const defaultPort uint32 = 1024

func main() {
	var cid uint32
	var port uint32

	root := &cobra.Command{
		Use:     "vex",
		Short:   "vex client — send VMP commands to vex-agent",
		Version: version.Version,
	}
	root.PersistentFlags().Uint32Var(&cid, "cid", 1, "vsocket context ID")
	root.PersistentFlags().Uint32Var(&port, "port", defaultPort, "vsocket port")

	root.AddCommand(
		pingCmd(&cid, &port),
		hostInfoCmd(&cid, &port),
		execCmd(&cid, &port),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func pingCmd(cid, port *uint32) *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Send a ping and expect pong",
		RunE: func(_ *cobra.Command, _ []string) error {
			c, err := client.New(*cid, *port)
			if err != nil {
				return err
			}
			defer c.Close()
			if err := c.Ping(); err != nil {
				return err
			}
			fmt.Println("pong")
			return nil
		},
	}
}

func hostInfoCmd(cid, port *uint32) *cobra.Command {
	return &cobra.Command{
		Use:   "host-info",
		Short: "Retrieve host OS information",
		RunE: func(_ *cobra.Command, _ []string) error {
			c, err := client.New(*cid, *port)
			if err != nil {
				return err
			}
			defer c.Close()
			hi, err := c.HostInfo()
			if err != nil {
				return err
			}
			out, _ := json.MarshalIndent(map[string]string{
				"os":      hi.OS,
				"version": hi.Version,
				"arch":    hi.Arch,
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
}

func execCmd(cid, port *uint32) *cobra.Command {
	var timeout uint32
	cmd := &cobra.Command{
		Use:   "exec <command>",
		Short: "Execute a command on the agent host",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := client.New(*cid, *port)
			if err != nil {
				return err
			}
			defer c.Close()
			ex, err := c.Exec(args[0], timeout)
			if err != nil {
				return err
			}
			if len(ex.Stdout) > 0 {
				fmt.Print(string(ex.Stdout))
			}
			if len(ex.Stderr) > 0 {
				fmt.Fprint(os.Stderr, string(ex.Stderr))
			}
			if ex.TimedOut {
				fmt.Fprintln(os.Stderr, "timed out")
			}
			if ex.ExitCode != 0 {
				os.Exit(int(ex.ExitCode))
			}
			return nil
		},
	}
	cmd.Flags().Uint32Var(&timeout, "timeout", 0, "timeout in seconds (0 = no timeout)")
	return cmd
}
