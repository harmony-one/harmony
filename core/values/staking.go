package values

import "github.com/pkg/errors"

// StakingDirective says what kind of payload follows
type StakingDirective byte

const (
	// DirectiveNewValidator ...
	DirectiveNewValidator StakingDirective = iota
	// DirectiveEditValidator ...
	DirectiveEditValidator
	// DirectiveDelegate ...
	DirectiveDelegate
	// DirectiveRedelegate ...
	DirectiveRedelegate
	// DirectiveUndelegate ...
	DirectiveUndelegate
)

var (
	directiveKind = [...]string{
		"NewValidator", "EditValidator", "Delegate", "Redelegate", "Undelegate",
	}
	// ErrInvalidStakingKind given when caller gives bad staking message kind
	ErrInvalidStakingKind = errors.New("bad staking kind")
)

func (d StakingDirective) String() string {
	return directiveKind[d]
}
