package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/jimbersoftware/vex/internal/vmp"
	"github.com/jimbersoftware/vex/internal/vsock"
	"github.com/spf13/cobra"
)

func main() {
	var cid uint32
	var port uint32

	root := &cobra.Command{
		Use:   "vex",
		Short: "vex client — send VMP commands to vex-agent",
	}
	root.PersistentFlags().Uint32Var(&cid, "cid", 1, "vsocket context ID")
	root.PersistentFlags().Uint32Var(&port, "port", 1024, "vsocket port")

	root.AddCommand(
		pingCmd(&cid, &port),
		hostInfoCmd(&cid, &port),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func connect(cid, port uint32) (net.Conn, *bufio.Reader, error) {
	conn, err := vsock.Dial(cid, port)
	if err != nil {
		return nil, nil, fmt.Errorf("connect cid=%d port=%d: %w", cid, port, err)
	}
	return conn, bufio.NewReader(conn), nil
}

func pingCmd(cid, port *uint32) *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Send a ping and expect pong",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, br, err := connect(*cid, *port)
			if err != nil {
				return err
			}
			defer conn.Close()

			req := &vmp.Request{Id: 1, Command: &vmp.Request_Ping{Ping: &vmp.PingRequest{}}}
			if err := vmp.WriteRequest(conn, req); err != nil {
				return err
			}
			resp, err := vmp.ReadResponse(br)
			if err != nil {
				return err
			}
			if resp.Error != "" {
				return fmt.Errorf("agent error: %s", resp.Error)
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
			conn, br, err := connect(*cid, *port)
			if err != nil {
				return err
			}
			defer conn.Close()

			req := &vmp.Request{Id: 1, Command: &vmp.Request_HostInfo{HostInfo: &vmp.HostInfoRequest{}}}
			if err := vmp.WriteRequest(conn, req); err != nil {
				return err
			}
			resp, err := vmp.ReadResponse(br)
			if err != nil {
				return err
			}
			if resp.Error != "" {
				return fmt.Errorf("agent error: %s", resp.Error)
			}
			hi := resp.Result.(*vmp.Response_HostInfo).HostInfo
			out, _ := json.MarshalIndent(map[string]string{
				"os":      hi.Os,
				"version": hi.Version,
				"arch":    hi.Arch,
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
}
