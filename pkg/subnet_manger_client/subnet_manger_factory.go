package subnet_manger_client

import (
	"fmt"
	"github.com/Mellanox/ib-sriov-cni/pkg/types"
)

func NewSubNetMangerClient(subnetMangerClientType string, connectDetails []byte) (types.SubnetMangerClient, error) {
	switch subnetMangerClientType {
	case "ufm":
		return newUfmSubnetMangerClient(connectDetails)
	default:
		return nil, fmt.Errorf("unknow subnet manger type %s", subnetMangerClientType)
	}
}
