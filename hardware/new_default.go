//go:build !pi

package hardware

func New() (*Devices, error) {
	return &Devices{
		Button: noopButton{},
		Ring:   noopRing{},
		Matrix: noopMatrix{},
	}, nil
}
