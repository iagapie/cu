package cu

// #include <cuda.h>
import "C"

// CUKernel is a CUDA kernel
type CUKernel struct {
	kern C.CUkernel
}

func (kern CUKernel) c() C.CUkernel { return kern.kern }
