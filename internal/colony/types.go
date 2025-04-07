package colony

import "time"

type Assets struct {
	ID string `json:"id"`
	// DatacenterID string     `json:"datacenter_id"`
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	CPUCores int       `json:"cpu_cores"`
	CPUArch  string    `json:"cpu_arch"`
	MemoryMB int       `json:"memory_mb"`
	Networks []Network `json:"networks"`
	Storages []Storage `json:"storages"`
	// CreatedAt    time.Time  `json:"created_at"`
	// UpdatedAt    time.Time  `json:"updated_at"`
	// DeletedAt    *time.Time `json:"deleted_at"`
}

// Network represents a network connection associated with a server
type Network struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	MacAddress  string    `json:"mac_address"`
	IPAddresses string    `json:"ip_addresses"`
	SubnetMask  string    `json:"subnet_mask"`
	AssetID     string    `json:"asset_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// Storage represents a storage device associated with a server
type Storage struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	SizeMB     int        `json:"size_mb"`
	IsPrimary  bool       `json:"is_primary"`
	MountPoint string     `json:"mount_point"`
	AssetID    string     `json:"asset_id"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at"`
}

type APIResponse[T any] struct {
	Data []T `json:"data"`
}
