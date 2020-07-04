package blsloader

import (
	"sync"

	"github.com/aws/aws-sdk-go/service/kms"
)

var (
	kmsClient   *kms.KMS
	kmsInitOnce sync.Once
)
