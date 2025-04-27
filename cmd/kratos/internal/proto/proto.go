package proto

import (
	"github.com/spf13/cobra"

	"github.com/yola1107/kratos/cmd/kratos/v2/internal/proto/add"
	"github.com/yola1107/kratos/cmd/kratos/v2/internal/proto/client"
	"github.com/yola1107/kratos/cmd/kratos/v2/internal/proto/server"
)

// CmdProto represents the proto command.
var CmdProto = &cobra.Command{
	Use:   "proto",
	Short: "Generate the proto files",
	Long:  "Generate the proto files.",
}

func init() {
	CmdProto.AddCommand(add.CmdAdd)
	CmdProto.AddCommand(client.CmdClient)
	CmdProto.AddCommand(server.CmdServer)
}
