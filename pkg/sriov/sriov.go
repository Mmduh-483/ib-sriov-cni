package sriov

import (
	"fmt"
	"github.com/Mellanox/sriovnet"
	"net"

	"github.com/containernetworking/plugins/pkg/ns"

	"github.com/Mellanox/ib-sriov-cni/pkg/types"
	"github.com/Mellanox/ib-sriov-cni/pkg/utils"
	"github.com/vishvananda/netlink"
)

// MyNetlink NetlinkManager
type MyNetlink struct {
}

// LinkByName implements NetlinkManager
func (n *MyNetlink) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

// LinkSetUp using NetlinkManager
func (n *MyNetlink) LinkSetUp(link netlink.Link) error {
	return netlink.LinkSetUp(link)
}

// LinkSetDown using NetlinkManager
func (n *MyNetlink) LinkSetDown(link netlink.Link) error {
	return netlink.LinkSetDown(link)
}

// LinkSetNsFd using NetlinkManager
func (n *MyNetlink) LinkSetNsFd(link netlink.Link, fd int) error {
	return netlink.LinkSetNsFd(link, fd)
}

// LinkSetName using NetlinkManager
func (n *MyNetlink) LinkSetName(link netlink.Link, name string) error {
	return netlink.LinkSetName(link, name)
}

// LinkSetVfState using NetlinkManager
func (n *MyNetlink) LinkSetVfState(link netlink.Link, vf int, state uint32) error {
	return netlink.LinkSetVfState(link, vf, state)
}

// LinkSetVfPortGUID using NetlinkManager
func (n *MyNetlink) LinkSetVfPortGUID(link netlink.Link, vf int, portGUID net.HardwareAddr) error {
	return netlink.LinkSetVfPortGUID(link, vf, portGUID)
}

// LinkSetVfNodeGUID using NetlinkManager
func (n *MyNetlink) LinkSetVfNodeGUID(link netlink.Link, vf int, nodeGUID net.HardwareAddr) error {
	return netlink.LinkSetVfNodeGUID(link, vf, nodeGUID)
}

type pciUtilsImpl struct{}

func (p *pciUtilsImpl) GetSriovNumVfs(ifName string) (int, error) {
	return utils.GetSriovNumVfs(ifName)
}

func (p *pciUtilsImpl) GetVFLinkNamesFromVFID(pfName string, vfID int) ([]string, error) {
	return utils.GetVFLinkNamesFromVFID(pfName, vfID)
}

func (p *pciUtilsImpl) GetPciAddress(ifName string, vf int) (string, error) {
	return utils.GetPciAddress(ifName, vf)
}

// RebindVf unbind then bind the vf
func (p *pciUtilsImpl) RebindVf(pfName, vfPciAddress string) error {
	pfHandle, err := sriovnet.GetPfNetdevHandle(pfName)
	if err != nil {
		return err
	}
	var vf *sriovnet.VfObj
	found := false
	for _, vfObj := range pfHandle.List {
		if vfObj.PciAddress == vfPciAddress {
			vf = vfObj
			found = true
		}
	}
	if !found {
		return fmt.Errorf("failed to find VF %s for PF %s", vfPciAddress, pfName)
	}
	if err = sriovnet.UnbindVf(pfHandle, vf); err != nil {
		return err
	}
	if err = sriovnet.BindVf(pfHandle, vf); err != nil {
		return err
	}
	return nil
}

type sriovManager struct {
	nLink types.NetlinkManager
	utils types.PciUtils
}

// NewSriovManager returns an instance of SriovManager
func NewSriovManager() types.Manager {
	return &sriovManager{
		nLink: &MyNetlink{},
		utils: &pciUtilsImpl{},
	}
}

// SetupVF sets up a VF in Pod netns
func (s *sriovManager) SetupVF(conf *types.NetConf, podifName string, cid string, netns ns.NetNS) error {
	linkName := conf.HostIFNames

	linkObj, err := s.nLink.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("error getting VF netdevice with name %s", linkName)
	}

	// tempName used as intermediary name to avoid name conflicts
	tempName := fmt.Sprintf("%s%d", linkName, linkObj.Attrs().Index)

	// 1. Set link down
	if err := s.nLink.LinkSetDown(linkObj); err != nil {
		return fmt.Errorf("failed to down vf device %q: %v", linkName, err)
	}

	// 2. Set temp name
	if err := s.nLink.LinkSetName(linkObj, tempName); err != nil {
		return fmt.Errorf("error setting temp IF name %s for %s", tempName, linkName)
	}

	// 3. Change netns
	if err := s.nLink.LinkSetNsFd(linkObj, int(netns.Fd())); err != nil {
		return fmt.Errorf("failed to move IF %s to netns: %q", tempName, err)
	}

	if err := netns.Do(func(_ ns.NetNS) error {
		// 4. Set Pod IF name
		if err := s.nLink.LinkSetName(linkObj, podifName); err != nil {
			return fmt.Errorf("error setting container interface name %s for %s", linkName, tempName)
		}

		// 5. Bring IF up in Pod netns
		if err := s.nLink.LinkSetUp(linkObj); err != nil {
			return fmt.Errorf("error bringing interface up in container ns: %q", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("error setting up interface in container namespace: %q", err)
	}
	conf.ContIFNames = podifName

	return nil
}

// ReleaseVF reset a VF from Pod netns and return it to init netns
func (s *sriovManager) ReleaseVF(conf *types.NetConf, podifName string, cid string, netns ns.NetNS) error {

	initns, err := ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("failed to get init netns: %v", err)
	}

	if len(conf.ContIFNames) < 1 && len(conf.ContIFNames) != len(conf.HostIFNames) {
		return fmt.Errorf("number of interface names mismatch ContIFNames: %d HostIFNames: %d", len(conf.ContIFNames), len(conf.HostIFNames))
	}

	return netns.Do(func(_ ns.NetNS) error {

		// get VF device
		linkObj, err := s.nLink.LinkByName(podifName)
		if err != nil {
			return fmt.Errorf("failed to get netlink device with name %s: %q", podifName, err)
		}

		// shutdown VF device
		if err = s.nLink.LinkSetDown(linkObj); err != nil {
			return fmt.Errorf("failed to set link %s down: %q", podifName, err)
		}

		// rename VF device
		err = s.nLink.LinkSetName(linkObj, conf.HostIFNames)
		if err != nil {
			return fmt.Errorf("failed to rename link %s to host name %s: %q", podifName, conf.HostIFNames, err)
		}

		// move VF device to init netns
		if err = s.nLink.LinkSetNsFd(linkObj, int(initns.Fd())); err != nil {
			return fmt.Errorf("failed to move interface %s to init netns: %v", conf.HostIFNames, err)
		}

		return nil
	})
}

// ApplyVFConfig configure a VF with parameters given in NetConf
func (s *sriovManager) ApplyVFConfig(conf *types.NetConf) error {

	pfLink, err := s.nLink.LinkByName(conf.Master)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.Master, err)
	}

	// Set link state
	if conf.LinkState != "" {
		var state uint32
		switch conf.LinkState {
		case "auto":
			state = netlink.VF_LINK_STATE_AUTO
		case "enable":
			state = netlink.VF_LINK_STATE_ENABLE
		case "disable":
			state = netlink.VF_LINK_STATE_DISABLE
		default:
			// the value should have been validated earlier, return error if we somehow got here
			return fmt.Errorf("unknown link state %s when setting it for vf %d: %v", conf.LinkState, conf.VFID, err)
		}
		if err = s.nLink.LinkSetVfState(pfLink, conf.VFID, state); err != nil {
			return fmt.Errorf("failed to set vf %d link state to %d: %v", conf.VFID, state, err)
		}
	}

	// Set link guid
	if conf.GUID != "" {
		if len(conf.GUID) < 22 || conf.GUID == "00:00:00:00:00:00:00:00" {
			return fmt.Errorf("invalid guid %s", conf.GUID)
		}
		// save link guid
		vfLink, err := s.nLink.LinkByName(conf.HostIFNames)
		if err != nil {
			return fmt.Errorf("failed to lookup vf %q: %v", conf.HostIFNames, err)
		}

		conf.HostIFGUID = vfLink.Attrs().HardwareAddr.String()[36:]

		// Set link guid
		guid, err := net.ParseMAC(conf.GUID)
		if err != nil {
			return fmt.Errorf("failed to parse guid %s: %v", conf.GUID, err)
		}
		if err = s.nLink.LinkSetVfNodeGUID(pfLink, conf.VFID, guid); err != nil {
			return fmt.Errorf("failed to add node guid %s: %v", guid, err)
		}
		if err = s.nLink.LinkSetVfPortGUID(pfLink, conf.VFID, guid); err != nil {
			return fmt.Errorf("failed to add port guid %s: %v", guid, err)
		}
		// unbind vf then bind it to apply the new guid
		if err = s.utils.RebindVf(conf.Master, conf.DeviceID); err != nil {
			return err
		}
	}

	return nil
}

// ResetVFConfig reset a VF with default values
func (s *sriovManager) ResetVFConfig(conf *types.NetConf) error {

	pfLink, err := s.nLink.LinkByName(conf.Master)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.Master, err)
	}

	// Reset link state to `auto`
	if conf.LinkState != "" {
		// While resetting to `auto` can be a reasonable thing to do regardless of whether it was explicitly
		// specified in the network definition, reset only when link_state was explicitly specified, to
		// accommodate for drivers / NICs that don't support the netlink command (e.g. igb driver)
		if err = s.nLink.LinkSetVfState(pfLink, conf.VFID, 0); err != nil {
			return fmt.Errorf("failed to set link state to auto for vf %d: %v", conf.VFID, err)
		}
	}

	// Reset link guid
	if conf.HostIFGUID != "" && conf.HostIFGUID != "00:00:00:00:00:00:00:00" {
		guid, err := net.ParseMAC(conf.HostIFGUID)
		if err != nil {
			return fmt.Errorf("failed to parse guid %s: %v", conf.HostIFGUID, err)
		}
		if err = s.nLink.LinkSetVfNodeGUID(pfLink, conf.VFID, guid); err != nil {
			return fmt.Errorf("failed to add node guid %s: %v", guid, err)
		}
		if err = s.nLink.LinkSetVfPortGUID(pfLink, conf.VFID, guid); err != nil {
			return fmt.Errorf("failed to add port guid %s: %v", guid, err)
		}
		// unbind vf then bind it to apply the guid
		if err = s.utils.RebindVf(conf.Master, conf.DeviceID); err != nil {
			return err
		}
	}

	return nil
}
