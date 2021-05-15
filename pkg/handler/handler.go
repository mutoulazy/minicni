package handler

import (
	"minicni/pkg/args"
	"minicni/pkg/nettool"
)

type Handler interface {
	HandleAdd(cmdArgs *args.CmdArgs) error
	HandleDel(cmdArgs *args.CmdArgs) error
	HandleCheck(cmdArgs *args.CmdArgs) error
	HandleVersion(cmdArgs *args.CmdArgs) error
}

type AddCmdResult struct {
	CniVersion string               `json:"cniVersion"`
	IPs        *nettool.AllocatedIP `json:"ips"`
}
