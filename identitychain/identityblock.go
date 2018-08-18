package identitychain

// IdentityBlock has the information of one node
type IdentityBlock struct {
	ID            int
	IP            string
	Port          string
	NumIdentities int32
}
