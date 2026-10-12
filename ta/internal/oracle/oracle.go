//go:build oracle

// Package oracle calls the reference TA-Lib C library (v0.8.1, built in
// third_party/ta-lib/build) through its abstract API, so tests can compare the
// Go port against C for any function by name.
//
// Build it with the "oracle" tag:  go test -tags oracle ./...
//
// All calls are serialised by a mutex because TA-Lib's settings are global.
package oracle

/*
#cgo CFLAGS: -I${SRCDIR}/../../../third_party/ta-lib/include
#cgo LDFLAGS: ${SRCDIR}/../../../third_party/ta-lib/build/libta-lib.a -lm
#include <stdlib.h>
#include "ta_libc.h"
*/
import "C"

import (
	"fmt"
	"math"
	"sync"
	"unsafe"
)

var (
	mu       sync.Mutex
	initOnce sync.Once
	initErr  error
)

func initialize() error {
	initOnce.Do(func() {
		if rc := C.TA_Initialize(); rc != C.TA_SUCCESS {
			initErr = fmt.Errorf("oracle: TA_Initialize failed: %d", int(rc))
		}
	})
	return initErr
}

// Return codes (TA_RetCode) that tests commonly assert on.
const (
	Success              = 0
	BadParam             = 2
	OutOfRangeStartIndex = 12
	OutOfRangeEndIndex   = 13
)

// Inputs are the input series for a call. Price components are used for
// TA_Input_Price parameters (only the ones the function needs must be set).
// Real is consumed, in order, by TA_Input_Real parameters; Integer by
// TA_Input_Integer parameters.
type Inputs struct {
	Open, High, Low, Close, Volume, OpenInterest []float64
	Real                                         [][]float64
	Integer                                      [][]int
}

// Output is one output parameter of a call.
type Output struct {
	Name  string
	IsInt bool
	Real  []float64 // raw C buffer, valid in [0, NbElement)
	Int   []int
}

// Result is what the C function returned.
type Result struct {
	RetCode   int
	BegIdx    int
	NbElement int
	Outputs   []Output
}

// AlignedReal returns output i index-aligned with an input of length n:
// value k of the C buffer moves to index BegIdx+k, everything else is NaN.
func (r Result) AlignedReal(i, n int) []float64 {
	out := make([]float64, n)
	for k := range out {
		out[k] = math.NaN()
	}
	if r.RetCode != Success {
		return out
	}
	for k := 0; k < r.NbElement; k++ {
		out[r.BegIdx+k] = r.Outputs[i].Real[k]
	}
	return out
}

// AlignedInt is AlignedReal for integer outputs; the padding is 0.
func (r Result) AlignedInt(i, n int) []int {
	out := make([]int, n)
	if r.RetCode != Success {
		return out
	}
	for k := 0; k < r.NbElement; k++ {
		out[r.BegIdx+k] = r.Outputs[i].Int[k]
	}
	return out
}

// Call runs function name over the whole input (startIdx 0, endIdx n-1).
// opt holds the optional inputs in declaration order; integer and MAType
// parameters are passed as whole-number float64s.
func Call(name string, in Inputs, opt ...float64) (Result, error) {
	n := inputLen(in)
	return CallRange(name, in, 0, n-1, opt...)
}

// CallRange runs function name for [startIdx, endIdx], like the C API.
func CallRange(name string, in Inputs, startIdx, endIdx int, opt ...float64) (Result, error) {
	mu.Lock()
	defer mu.Unlock()
	if err := initialize(); err != nil {
		return Result{}, err
	}
	var allocs []unsafe.Pointer
	defer func() {
		for _, p := range allocs {
			C.free(p)
		}
	}()
	cReal := func(s []float64) *C.double {
		if s == nil {
			return nil
		}
		size := C.size_t(len(s)+1) * C.size_t(unsafe.Sizeof(C.double(0)))
		p := C.malloc(size)
		allocs = append(allocs, p)
		dst := unsafe.Slice((*C.double)(p), len(s)+1)
		for i, v := range s {
			dst[i] = C.double(v)
		}
		return (*C.double)(p)
	}

	h, info, err := handle(name)
	if err != nil {
		return Result{}, err
	}
	var ph *C.TA_ParamHolder
	if rc := C.TA_ParamHolderAlloc(h, &ph); rc != C.TA_SUCCESS {
		return Result{}, fmt.Errorf("oracle: %s: TA_ParamHolderAlloc: %d", name, int(rc))
	}
	defer C.TA_ParamHolderFree(ph)

	realIdx, intIdx := 0, 0
	for i := 0; i < int(info.nbInput); i++ {
		var pi *C.TA_InputParameterInfo
		C.TA_GetInputParameterInfo(h, C.uint(i), &pi)
		var rc C.TA_RetCode
		switch pi._type {
		case C.TA_Input_Price:
			rc = C.TA_SetInputParamPricePtr(ph, C.uint(i), cReal(in.Open), cReal(in.High), cReal(in.Low),
				cReal(in.Close), cReal(in.Volume), cReal(in.OpenInterest))
		case C.TA_Input_Real:
			if realIdx >= len(in.Real) {
				return Result{}, fmt.Errorf("oracle: %s: needs Real input #%d", name, realIdx)
			}
			rc = C.TA_SetInputParamRealPtr(ph, C.uint(i), cReal(in.Real[realIdx]))
			realIdx++
		case C.TA_Input_Integer:
			if intIdx >= len(in.Integer) {
				return Result{}, fmt.Errorf("oracle: %s: needs Integer input #%d", name, intIdx)
			}
			src := in.Integer[intIdx]
			p := C.malloc(C.size_t(len(src)+1) * C.size_t(unsafe.Sizeof(C.int(0))))
			allocs = append(allocs, p)
			dst := unsafe.Slice((*C.int)(p), len(src)+1)
			for k, v := range src {
				dst[k] = C.int(v)
			}
			rc = C.TA_SetInputParamIntegerPtr(ph, C.uint(i), (*C.int)(p))
			intIdx++
		}
		if rc != C.TA_SUCCESS {
			return Result{RetCode: int(rc)}, nil
		}
	}

	if len(opt) != int(info.nbOptInput) {
		return Result{}, fmt.Errorf("oracle: %s takes %d optional inputs, got %d", name, int(info.nbOptInput), len(opt))
	}
	for i, v := range opt {
		var pi *C.TA_OptInputParameterInfo
		C.TA_GetOptInputParameterInfo(h, C.uint(i), &pi)
		var rc C.TA_RetCode
		switch pi._type {
		case C.TA_OptInput_RealRange, C.TA_OptInput_RealList:
			rc = C.TA_SetOptInputParamReal(ph, C.uint(i), C.double(v))
		default:
			rc = C.TA_SetOptInputParamInteger(ph, C.uint(i), C.int(int(v)))
		}
		if rc != C.TA_SUCCESS {
			return Result{RetCode: int(rc)}, nil
		}
	}

	n := endIdx + 1
	if n < 1 {
		n = 1
	}
	res := Result{}
	cOuts := make([]unsafe.Pointer, int(info.nbOutput))
	for i := 0; i < int(info.nbOutput); i++ {
		var po *C.TA_OutputParameterInfo
		C.TA_GetOutputParameterInfo(h, C.uint(i), &po)
		o := Output{Name: C.GoString(po.paramName)}
		var rc C.TA_RetCode
		if po._type == C.TA_Output_Integer {
			o.IsInt = true
			p := C.malloc(C.size_t(n) * C.size_t(unsafe.Sizeof(C.int(0))))
			allocs = append(allocs, p)
			cOuts[i] = p
			rc = C.TA_SetOutputParamIntegerPtr(ph, C.uint(i), (*C.int)(p))
		} else {
			p := C.malloc(C.size_t(n) * C.size_t(unsafe.Sizeof(C.double(0))))
			allocs = append(allocs, p)
			cOuts[i] = p
			rc = C.TA_SetOutputParamRealPtr(ph, C.uint(i), (*C.double)(p))
		}
		if rc != C.TA_SUCCESS {
			return Result{RetCode: int(rc)}, nil
		}
		res.Outputs = append(res.Outputs, o)
	}

	var beg, nb C.int
	rc := C.TA_CallFunc(ph, C.int(startIdx), C.int(endIdx), &beg, &nb)
	res.RetCode, res.BegIdx, res.NbElement = int(rc), int(beg), int(nb)
	if rc != C.TA_SUCCESS {
		return res, nil
	}
	for i := range res.Outputs {
		if res.Outputs[i].IsInt {
			src := unsafe.Slice((*C.int)(cOuts[i]), n)
			res.Outputs[i].Int = make([]int, res.NbElement)
			for k := range res.Outputs[i].Int {
				res.Outputs[i].Int[k] = int(src[k])
			}
		} else {
			src := unsafe.Slice((*C.double)(cOuts[i]), n)
			res.Outputs[i].Real = make([]float64, res.NbElement)
			for k := range res.Outputs[i].Real {
				res.Outputs[i].Real[k] = float64(src[k])
			}
		}
	}
	return res, nil
}

// Lookback returns TA_GetLookback for name with the given optional inputs,
// or the TA_RetCode as an error.
func Lookback(name string, opt ...float64) (int, error) {
	mu.Lock()
	defer mu.Unlock()
	if err := initialize(); err != nil {
		return 0, err
	}
	h, info, err := handle(name)
	if err != nil {
		return 0, err
	}
	var ph *C.TA_ParamHolder
	if rc := C.TA_ParamHolderAlloc(h, &ph); rc != C.TA_SUCCESS {
		return 0, fmt.Errorf("oracle: %s: TA_ParamHolderAlloc: %d", name, int(rc))
	}
	defer C.TA_ParamHolderFree(ph)
	if len(opt) != int(info.nbOptInput) {
		return 0, fmt.Errorf("oracle: %s takes %d optional inputs, got %d", name, int(info.nbOptInput), len(opt))
	}
	for i, v := range opt {
		var pi *C.TA_OptInputParameterInfo
		C.TA_GetOptInputParameterInfo(h, C.uint(i), &pi)
		var rc C.TA_RetCode
		switch pi._type {
		case C.TA_OptInput_RealRange, C.TA_OptInput_RealList:
			rc = C.TA_SetOptInputParamReal(ph, C.uint(i), C.double(v))
		default:
			rc = C.TA_SetOptInputParamInteger(ph, C.uint(i), C.int(int(v)))
		}
		if rc != C.TA_SUCCESS {
			return 0, fmt.Errorf("oracle: %s: set opt %d: retcode %d", name, i, int(rc))
		}
	}
	var lb C.int
	if rc := C.TA_GetLookback(ph, &lb); rc != C.TA_SUCCESS {
		return 0, fmt.Errorf("oracle: %s: TA_GetLookback: retcode %d", name, int(rc))
	}
	return int(lb), nil
}

// SetUnstablePeriod sets a C unstable period (id as in ta.FuncUnstID).
func SetUnstablePeriod(id, period int) {
	mu.Lock()
	defer mu.Unlock()
	_ = initialize()
	C.TA_SetUnstablePeriod(C.TA_FuncUnstId(id), C.uint(period))
}

// ResetUnstablePeriods sets every C unstable period back to 0.
func ResetUnstablePeriods() {
	mu.Lock()
	defer mu.Unlock()
	_ = initialize()
	C.TA_SetUnstablePeriod(C.TA_FUNC_UNST_ALL, 0)
}

// SetCandleSettings overrides one C candle setting (types as in ta).
func SetCandleSettings(settingType, rangeType, avgPeriod int, factor float64) {
	mu.Lock()
	defer mu.Unlock()
	_ = initialize()
	C.TA_SetCandleSettings(C.TA_CandleSettingType(settingType), C.TA_RangeType(rangeType), C.int(avgPeriod), C.double(factor))
}

// RestoreCandleDefaults restores every C candle setting.
func RestoreCandleDefaults() {
	mu.Lock()
	defer mu.Unlock()
	_ = initialize()
	C.TA_RestoreCandleDefaultSettings(C.TA_AllCandleSettings)
}

func handle(name string) (*C.TA_FuncHandle, *C.TA_FuncInfo, error) {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	var h *C.TA_FuncHandle
	if rc := C.TA_GetFuncHandle(cs, &h); rc != C.TA_SUCCESS {
		return nil, nil, fmt.Errorf("oracle: unknown function %q (retcode %d)", name, int(rc))
	}
	var info *C.TA_FuncInfo
	if rc := C.TA_GetFuncInfo(h, &info); rc != C.TA_SUCCESS {
		return nil, nil, fmt.Errorf("oracle: %s: TA_GetFuncInfo: %d", name, int(rc))
	}
	return h, info, nil
}

func inputLen(in Inputs) int {
	for _, s := range [][]float64{in.Close, in.Open, in.High, in.Low, in.Volume, in.OpenInterest} {
		if s != nil {
			return len(s)
		}
	}
	if len(in.Real) > 0 {
		return len(in.Real[0])
	}
	if len(in.Integer) > 0 {
		return len(in.Integer[0])
	}
	return 0
}

// Input kinds reported by Info.
const (
	InputPrice   = 0
	InputReal    = 1
	InputInteger = 2
)

// Price component flags of a price input (TA_IN_PRICE_*).
const (
	PriceOpen         = 0x01
	PriceHigh         = 0x02
	PriceLow          = 0x04
	PriceClose        = 0x08
	PriceVolume       = 0x10
	PriceOpenInterest = 0x20
)

// InputInfo describes one input of a function.
type InputInfo struct {
	Kind  int // InputPrice, InputReal or InputInteger
	Flags int // price component flags for InputPrice
}

// OptInfo describes one optional input. For an integer list (MAType) Min
// and Max span the listed values and List holds them.
type OptInfo struct {
	Name     string
	IsReal   bool
	Min, Max float64
	Default  float64
	List     []int
}

// FuncInfo is the abstract-API description of a function.
type FuncInfo struct {
	Inputs     []InputInfo
	Opts       []OptInfo
	NbOutputs  int
	IntOutputs []bool
}

// Info returns the abstract-API description of name.
func Info(name string) (FuncInfo, error) {
	mu.Lock()
	defer mu.Unlock()
	if err := initialize(); err != nil {
		return FuncInfo{}, err
	}
	h, info, err := handle(name)
	if err != nil {
		return FuncInfo{}, err
	}
	var fi FuncInfo
	for i := 0; i < int(info.nbInput); i++ {
		var pi *C.TA_InputParameterInfo
		C.TA_GetInputParameterInfo(h, C.uint(i), &pi)
		ii := InputInfo{Flags: int(pi.flags)}
		switch pi._type {
		case C.TA_Input_Price:
			ii.Kind = InputPrice
		case C.TA_Input_Real:
			ii.Kind = InputReal
		default:
			ii.Kind = InputInteger
		}
		fi.Inputs = append(fi.Inputs, ii)
	}
	for i := 0; i < int(info.nbOptInput); i++ {
		var pi *C.TA_OptInputParameterInfo
		C.TA_GetOptInputParameterInfo(h, C.uint(i), &pi)
		oi := OptInfo{Name: C.GoString(pi.paramName), Default: float64(pi.defaultValue)}
		switch pi._type {
		case C.TA_OptInput_RealRange:
			r := (*C.TA_RealRange)(pi.dataSet)
			oi.IsReal, oi.Min, oi.Max = true, float64(r.min), float64(r.max)
		case C.TA_OptInput_IntegerRange:
			r := (*C.TA_IntegerRange)(pi.dataSet)
			oi.Min, oi.Max = float64(r.min), float64(r.max)
		case C.TA_OptInput_IntegerList:
			l := (*C.TA_IntegerList)(pi.dataSet)
			pairs := unsafe.Slice(l.data, int(l.nbElement))
			oi.Min, oi.Max = math.Inf(1), math.Inf(-1)
			for _, p := range pairs {
				v := int(p.value)
				oi.List = append(oi.List, v)
				oi.Min, oi.Max = math.Min(oi.Min, float64(v)), math.Max(oi.Max, float64(v))
			}
		default:
			return FuncInfo{}, fmt.Errorf("oracle: %s: unsupported opt input type %d", name, int(pi._type))
		}
		fi.Opts = append(fi.Opts, oi)
	}
	fi.NbOutputs = int(info.nbOutput)
	for i := 0; i < fi.NbOutputs; i++ {
		var po *C.TA_OutputParameterInfo
		C.TA_GetOutputParameterInfo(h, C.uint(i), &po)
		fi.IntOutputs = append(fi.IntOutputs, po._type == C.TA_Output_Integer)
	}
	return fi, nil
}

// SetAllUnstablePeriods sets every function's unstable period.
func SetAllUnstablePeriods(period int) {
	mu.Lock()
	defer mu.Unlock()
	_ = initialize()
	C.TA_SetUnstablePeriod(C.TA_FUNC_UNST_ALL, C.uint(period))
}
