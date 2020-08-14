package common

// Error for rosetta error responses
type Error int32

const (
	// CatchAllError ..
	CatchAllError Error = iota
)

// Code ..
func (e Error) Code() int32 {
	return int32(e)
}
