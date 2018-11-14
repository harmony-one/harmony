package ida

// HarmonyIDA implements IDA interface.
type HarmonyIDA struct {
	raptorQImp *RaptorQ
}

// TakeRaptorQ takes RaptorQ implementation.
func (ida *HarmonyIDA) TakeRaptorQ(raptorQImp *RaptorQ) {
	ida.raptorQImp = raptorQImp
}
