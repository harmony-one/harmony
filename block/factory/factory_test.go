package blockfactory

import (
	"math/big"
	"testing"

	"github.com/harmony-one/harmony/internal/params"
)

func Test_factory_NewHeader(t *testing.T) {
	type fields struct {
		chainConfig *params.ChainConfig
	}
	type args struct {
		epoch *big.Int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"MainnetGenesis",
			fields{params.MainnetChainConfig},
			args{big.NewInt(0)},
		},
		{
			"MainnetEpoch1",
			fields{params.MainnetChainConfig},
			args{big.NewInt(1)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &factory{
				chainConfig: tt.fields.chainConfig,
			}
			got := f.NewHeader(tt.args.epoch)
			gotEpoch := got.Epoch()
			if gotEpoch.Cmp(tt.args.epoch) != 0 {
				t.Errorf("NewHeader() got epoch %s, want %s",
					gotEpoch, tt.args.epoch)
			}
		})
	}
}
