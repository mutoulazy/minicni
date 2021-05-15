package handler

import (
	"encoding/json"
	"fmt"
	"github.com/containernetworking/plugins/pkg/ns"
	"io/ioutil"
	"minicni/pkg/args"
	"minicni/pkg/nettool"
	"minicni/pkg/version"
	"os"
	"strings"
)

type FileHandler struct {
	*version.VersionInfo
	IPStore string
}

func (f *FileHandler) HandleAdd(cmdArgs *args.CmdArgs) error {
	cniConfig := args.CNIConfiguration{}
	if err := json.Unmarshal(cmdArgs.StdinData, &cniConfig); err != nil {
		return err
	}
	allIPs, err := nettool.GetAllIPs(cniConfig.Subnet)
	if err != nil {
		return err
	}
	gwIP := allIPs[0]

	// open or create the file that stores all the reserved IPs
	file, err := os.OpenFile(f.IPStore, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("failed to open file that stores reserved IPs %v", err)
	}
	defer file.Close()
	// get all the reserved IPs from file
	content, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	reservedIPs := strings.Split(strings.TrimSpace(string(content)), "\n")

	// allocate pod ip
	podIP := ""
	for _, ip := range allIPs[1:] {
		reserved := false
		for _, rip := range reservedIPs {
			if ip == rip {
				reserved = true
				break
			}
		}
		if !reserved {
			podIP = ip
			reservedIPs = append(reservedIPs, podIP)
			break
		}
	}
	if podIP == "" {
		return fmt.Errorf("no IP available")
	}

	// Create or update bridge
	brName := cniConfig.Bridge
	if brName != "" {
		// fall back to default bridge name: minicni0
		brName = "minicni0"
	}
	mtu := cniConfig.MTU
	if mtu == 0 {
		// fall back to default MTU: 1500
		mtu = 1500
	}
	br, err := nettool.CreateOrUpdateBridge(brName, gwIP, mtu)
	if err != nil {
		return err
	}

	netns, err := ns.GetNS(cmdArgs.Netns)
	if err != nil {
		return err
	}
	if err := nettool.SetupVeth(netns, br, cmdArgs.IfName, podIP, gwIP, mtu); err != nil {
		return err
	}

	// write reserved IPs back into file
	if err := ioutil.WriteFile(f.IPStore, []byte(strings.Join(reservedIPs, "\n")), 0600); err != nil {
		return fmt.Errorf("failed to write reserved IPs into file: %v", err)
	}

	addCmdResult := &AddCmdResult{
		CniVersion: cniConfig.CniVersion,
		IPs: &nettool.AllocatedIP{
			Version: "IPv4",
			Address: podIP,
			Gateway: gwIP,
		},
	}
	addCmdResultBytes, err := json.Marshal(addCmdResult)
	if err != nil {
		return err
	}
	// kubelet expects json format from stdout if success
	fmt.Print(string(addCmdResultBytes))

	return nil
}

func (f *FileHandler) HandleDel(cmdArgs *args.CmdArgs) error {
	netns, err := ns.GetNS(cmdArgs.Netns)
	if err != nil {
		return err
	}
	ip, err := nettool.GetVethIPInNS(netns, cmdArgs.IfName)
	if err != nil {
		return err
	}

	// open or create the file that stores all the reserved IPs
	file, err := os.OpenFile(f.IPStore, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("failed to open file that stores reserved IPs %v", err)
	}
	defer file.Close()

	// get all the reserved IPs from file
	content, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	reservedIPs := strings.Split(strings.TrimSpace(string(content)), "\n")

	for i, rip := range reservedIPs {
		if rip == ip {
			reservedIPs = append(reservedIPs[:i], reservedIPs[i+1:]...)
			break
		}
	}

	// write reserved IPs back into file
	if err := ioutil.WriteFile(f.IPStore, []byte(strings.Join(reservedIPs, "\n")), 0600); err != nil {
		return fmt.Errorf("failed to write reserved IPs into file: %v", err)
	}

	return nil
}

func (f *FileHandler) HandleCheck(cmdArgs *args.CmdArgs) error {
	// TODO
	return nil
}

func (f *FileHandler) HandleVersion(cmdArgs *args.CmdArgs) error {
	versionInfo, err := json.Marshal(f.VersionInfo)
	if err != nil {
		return err
	}
	fmt.Print(string(versionInfo))
	return nil
}

func NewFileHandler(filename string) Handler {
	return &FileHandler{
		VersionInfo: &version.VersionInfo{
			CniVersion:        version.Version,
			SupportedVersions: []string{version.Version},
		},
		IPStore: filename,
	}
}
