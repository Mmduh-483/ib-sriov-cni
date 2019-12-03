package sriov

import (
	"github.com/Mellanox/ib-sriov-cni/pkg/types"
	"github.com/Mellanox/ib-sriov-cni/pkg/types/mocks"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/testutils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
	"net"
)

// FakeLink is a dummy netlink struct used during testing
type FakeLink struct {
	netlink.LinkAttrs
}

// type FakeLink struct {
// 	linkAtrrs *netlink.LinkAttrs
// }

func (l *FakeLink) Attrs() *netlink.LinkAttrs {
	return &l.LinkAttrs
}

func (l *FakeLink) Type() string {
	return "FakeLink"
}

var _ = Describe("Sriov", func() {

	Context("Checking ApplyVFConfig function", func() {
		var (
			netconf *types.NetConf
		)

		BeforeEach(func() {
			netconf = &types.NetConf{
				Master:      "i4",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
				HostIFNames: "i1",
			}
		})

		It("ApplyVFConfig without GUID", func() {
			mocked := &mocks.NetlinkManager{}

			fakeLink := &FakeLink{netlink.LinkAttrs{}}

			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetVfState", fakeLink, mock.AnythingOfType("int"), mock.AnythingOfType("unit32")).Return(nil)
			sm := sriovManager{nLink: mocked}
			err := sm.ApplyVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
		})
		It("ApplyVFConfig with valid GUID", func() {
			mockedNetLinkManger := &mocks.NetlinkManager{}
			mockedPciUtils := &mocks.PciUtils{}

			gid, err := net.ParseMAC("00:00:04:a5:fe:80:00:00:00:00:00:00:11:22:33:00:00:aa:bb:cc")
			Expect(err).ToNot(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				HardwareAddr: gid,
			}}
			netconf.GUID = "01:23:45:67:89:ab:cd:ef"

			mockedNetLinkManger.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mockedNetLinkManger.On("LinkSetVfNodeGUID", fakeLink, mock.AnythingOfType("int"), mock.Anything).Return(nil)
			mockedNetLinkManger.On("LinkSetVfPortGUID", fakeLink, mock.AnythingOfType("int"), mock.Anything).Return(nil)

			mockedPciUtils.On("RebindVf", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

			sm := sriovManager{nLink: mockedNetLinkManger, utils: mockedPciUtils}
			err = sm.ApplyVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
		})
		It("ApplyVFConfig with invalid GUID", func() {
			mockedNetLinkManger := &mocks.NetlinkManager{}
			mockedPciUtils := &mocks.PciUtils{}

			gid, err := net.ParseMAC("00:00:04:a5:fe:80:00:00:00:00:00:00:11:22:33:00:00:aa:bb:cc")
			Expect(err).ToNot(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				HardwareAddr: gid,
			}}
			netconf.GUID = "invalid GUID"

			mockedNetLinkManger.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)

			sm := sriovManager{nLink: mockedNetLinkManger, utils: mockedPciUtils}
			err = sm.ApplyVFConfig(netconf)
			Expect(err).To(HaveOccurred())
		})
	})
	Context("Checking SetupVF function", func() {
		var (
			podifName string
			contID    string
			netconf   *types.NetConf
		)

		BeforeEach(func() {
			podifName = "net1"
			contID = "dummycid"
			netconf = &types.NetConf{
				Master:      "ib0",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
				HostIFNames: "ib1",
				ContIFNames: "net1",
			}
		})

		It("Assuming existing interface", func() {
			var targetNetNS ns.NetNS
			targetNetNS, err := testutils.NewNS()
			defer func() {
				if targetNetNS != nil {
					targetNetNS.Close()
				}
			}()
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks.NetlinkManager{}

			Expect(err).NotTo(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				Index: 1000,
				Name:  "dummylink",
			}}

			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, mock.Anything).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			mocked.On("LinkSetUp", fakeLink).Return(nil)
			sm := sriovManager{nLink: mocked}
			err = sm.SetupVF(netconf, podifName, contID, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("Checking ReleaseVF function", func() {
		var (
			podifName string
			contID    string
			netconf   *types.NetConf
		)

		BeforeEach(func() {
			podifName = "net1"
			contID = "dummycid"
			netconf = &types.NetConf{
				Master:      "ib0",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
				HostIFNames: "ib1",
				ContIFNames: "net1",
			}
		})
		It("Assuming existing interface", func() {
			var targetNetNS ns.NetNS
			targetNetNS, err := testutils.NewNS()
			defer func() {
				if targetNetNS != nil {
					targetNetNS.Close()
				}
			}()
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks.NetlinkManager{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, mock.Anything).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			mocked.On("LinkSetUp", fakeLink).Return(nil)
			sm := sriovManager{nLink: mocked}
			err = sm.ReleaseVF(netconf, podifName, contID, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("Checking ResetVFConfig function", func() {
		var (
			netconf *types.NetConf
		)

		BeforeEach(func() {
			netconf = &types.NetConf{
				Master:      "i4",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
				HostIFNames: "i1",
			}
		})

		It("ResetVFConfig without GUID", func() {
			mocked := &mocks.NetlinkManager{}

			fakeLink := &FakeLink{netlink.LinkAttrs{}}

			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetVfState", fakeLink, mock.AnythingOfType("int"), mock.AnythingOfType("unit32")).Return(nil)
			sm := sriovManager{nLink: mocked}
			err := sm.ResetVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
		})
		It("ResetVFConfig with valid GUID", func() {
			mockedNetLinkManger := &mocks.NetlinkManager{}
			mockedPciUtils := &mocks.PciUtils{}

			fakeLink := &FakeLink{netlink.LinkAttrs{}}
			netconf.HostIFGUID = "01:23:45:67:89:ab:cd:ef"

			mockedNetLinkManger.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mockedNetLinkManger.On("LinkSetVfNodeGUID", fakeLink, mock.AnythingOfType("int"), mock.Anything).Return(nil)
			mockedNetLinkManger.On("LinkSetVfPortGUID", fakeLink, mock.AnythingOfType("int"), mock.Anything).Return(nil)

			mockedPciUtils.On("RebindVf", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

			sm := sriovManager{nLink: mockedNetLinkManger, utils: mockedPciUtils}
			err := sm.ResetVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
		})
		It("ResetVFConfig with invalid GUID", func() {
			mockedNetLinkManger := &mocks.NetlinkManager{}
			mockedPciUtils := &mocks.PciUtils{}

			fakeLink := &FakeLink{netlink.LinkAttrs{}}
			netconf.HostIFGUID = "invalid GUID"

			mockedNetLinkManger.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)

			sm := sriovManager{nLink: mockedNetLinkManger, utils: mockedPciUtils}
			err := sm.ResetVFConfig(netconf)
			Expect(err).To(HaveOccurred())
		})
	})
})
