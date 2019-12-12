package types

import (
	"encoding/json"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"net"
)

// NetConf extends types.NetConf for ib-sriov-cni
type NetConf struct {
	types.NetConf
	Master             string
	DeviceID           string `json:"deviceID"` // PCI address of a VF in valid sysfs format
	VFID               int
	HostIFNames        string // VF netdevice name(s)
	HostIFGUID         string // VF netdevice GUID
	ContIFNames        string // VF names after in the container; used during deletion
	ActiveGUID         string
	GUID               string          `json:"guid"`
	PKey               string          `json:"pkey"`
	LinkState          string          `json:"link_state,omitempty"` // auto|enable|disable
	SubnetMangerClient string          `json:"subnetManger"`
	SubnetMangerConfig json.RawMessage `json:"subnetMangerConfig"`
}

// Manager provides interface invoke sriov nic related operations
type Manager interface {
	SetupVF(conf *NetConf, podifName string, cid string, netns ns.NetNS) error
	ReleaseVF(conf *NetConf, podifName string, cid string, netns ns.NetNS) error
	ResetVFConfig(conf *NetConf) error
	ApplyVFConfig(conf *NetConf) error
}

// mocked netlink interface
// required for unit tests

// NetlinkManager is an interface to mock nelink library
type NetlinkManager interface {
	LinkByName(string) (netlink.Link, error)
	LinkSetUp(netlink.Link) error
	LinkSetDown(netlink.Link) error
	LinkSetNsFd(netlink.Link, int) error
	LinkSetName(netlink.Link, string) error
	LinkSetVfState(netlink.Link, int, uint32) error
	LinkSetVfPortGUID(netlink.Link, int, net.HardwareAddr) error
	LinkSetVfNodeGUID(netlink.Link, int, net.HardwareAddr) error
}

// PciUtils is interface to help in SR-IOV functions
type PciUtils interface {
	GetSriovNumVfs(ifName string) (int, error)
	GetVFLinkNamesFromVFID(pfName string, vfID int) ([]string, error)
	GetPciAddress(ifName string, vf int) (string, error)
	RebindVf(pfName, vfPciAddress string) error
}

// SubnetManagerClient is interface to help connecting Infiniband subnet manger
type SubnetMangerClient interface {
	Connect() error
	AddPKey(string, string) error
	RemovePKey(string, string) error
}
