package fs

import "fmt"

const (
	CapList uint64 = 1 << iota
	CapRead
	CapWrite
	CapMkdir
	CapRename
	CapRemove
)

func HasCapability(fsys FileSystem, capability uint64) bool {
	if fsys == nil {
		return false
	}
	return fsys.Capabilities()&capability != 0
}

func CapabilityLabel(capability uint64) string {
	switch capability {
	case CapList:
		return "list"
	case CapRead:
		return "read"
	case CapWrite:
		return "write"
	case CapMkdir:
		return "mkdir"
	case CapRename:
		return "rename"
	case CapRemove:
		return "remove"
	default:
		return "unknown"
	}
}

func CapabilityError(uri URI, capability uint64) error {
	return fmt.Errorf("%s does not support %s", uri.Scheme, CapabilityLabel(capability))
}
