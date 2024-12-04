package machines

import (
	"github.com/stretchr/testify/mock"
)

type machinesMock struct {
	mock.Mock
	Machines
}
