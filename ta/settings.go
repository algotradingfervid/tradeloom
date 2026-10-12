package ta

import "sync"

// MAType selects a moving average. Values match TA_MAType in TA-Lib.
type MAType int

const (
	MATypeSMA      MAType = 0
	MATypeEMA      MAType = 1
	MATypeWMA      MAType = 2
	MATypeDEMA     MAType = 3
	MATypeTEMA     MAType = 4
	MATypeTRIMA    MAType = 5
	MATypeKAMA     MAType = 6
	MATypeMAMA     MAType = 7
	MATypeT3       MAType = 8
	MATypeHMA      MAType = 9
	MATypeDisabled MAType = 10 // identity: output equals input (see MA)
	MATypeDefault  MAType = 11 // substituted by each function's documented default
	MATypeZLEMA    MAType = 12
	MATypeRMA      MAType = 13
)

// FuncUnstID identifies a function whose output has an unstable period.
// Values match TA_FuncUnstId in TA-Lib.
type FuncUnstID int

const (
	UnstADX         FuncUnstID = 0
	UnstATR         FuncUnstID = 2
	UnstCMO         FuncUnstID = 3
	UnstDX          FuncUnstID = 4
	UnstEMA         FuncUnstID = 5
	UnstHTDCPeriod  FuncUnstID = 6
	UnstHTDCPhase   FuncUnstID = 7
	UnstHTPhasor    FuncUnstID = 8
	UnstHTSine      FuncUnstID = 9
	UnstHTTrendline FuncUnstID = 10
	UnstHTTrendMode FuncUnstID = 11
	UnstKAMA        FuncUnstID = 13
	UnstMAMA        FuncUnstID = 14
	UnstMinusDI     FuncUnstID = 16
	UnstMinusDM     FuncUnstID = 17
	UnstNATR        FuncUnstID = 18
	UnstPlusDI      FuncUnstID = 19
	UnstPlusDM      FuncUnstID = 20
	UnstRSI         FuncUnstID = 21
	UnstT3          FuncUnstID = 23
	UnstRMA         FuncUnstID = 24
	UnstHA          FuncUnstID = 25
	UnstRVI         FuncUnstID = 26

	unstCount = 27
)

// RangeType is the part of a candle measured by a CandleSetting.
type RangeType int

const (
	RangeRealBody RangeType = 0 // |close - open|
	RangeHighLow  RangeType = 1 // high - low
	RangeShadows  RangeType = 2 // upper shadow + lower shadow
)

// CandleSettingType names a candle-size threshold used by the CDL* patterns.
type CandleSettingType int

const (
	BodyLong        CandleSettingType = 0
	BodyVeryLong    CandleSettingType = 1
	BodyShort       CandleSettingType = 2
	BodyDoji        CandleSettingType = 3
	ShadowLong      CandleSettingType = 4
	ShadowVeryLong  CandleSettingType = 5
	ShadowShort     CandleSettingType = 6
	ShadowVeryShort CandleSettingType = 7
	Near            CandleSettingType = 8
	Far             CandleSettingType = 9
	Equal           CandleSettingType = 10

	candleSettingCount = 11
)

// CandleSetting defines when a candle part counts as e.g. "long": compared
// with Factor times the average RangeType over the previous AvgPeriod candles
// (or the current candle when AvgPeriod is 0).
type CandleSetting struct {
	RangeType RangeType
	AvgPeriod int
	Factor    float64
}

// defaultCandleSettings mirrors TA_RestoreCandleDefaultSettings (ta_global.c).
var defaultCandleSettings = [candleSettingCount]CandleSetting{
	BodyLong:        {RangeRealBody, 10, 1.0},
	BodyVeryLong:    {RangeRealBody, 10, 3.0},
	BodyShort:       {RangeRealBody, 10, 1.0},
	BodyDoji:        {RangeHighLow, 10, 0.1},
	ShadowLong:      {RangeRealBody, 0, 1.0},
	ShadowVeryLong:  {RangeRealBody, 0, 2.0},
	ShadowShort:     {RangeShadows, 10, 1.0},
	ShadowVeryShort: {RangeHighLow, 10, 0.1},
	Near:            {RangeHighLow, 5, 0.2},
	Far:             {RangeHighLow, 5, 0.6},
	Equal:           {RangeHighLow, 5, 0.05},
}

var settings = struct {
	sync.RWMutex
	unstable [unstCount]int
	candle   [candleSettingCount]CandleSetting
}{candle: defaultCandleSettings}

// SetUnstablePeriod sets the extra leading bars skipped by an indicator with
// an unstable period (TA_SetUnstablePeriod). period must be >= 0.
func SetUnstablePeriod(id FuncUnstID, period int) error {
	if id < 0 || id >= unstCount {
		return badParam("SetUnstablePeriod", "id", "unknown FuncUnstID %d", int(id))
	}
	if period < 0 {
		return badParam("SetUnstablePeriod", "period", "%d is negative", period)
	}
	settings.Lock()
	settings.unstable[id] = period
	settings.Unlock()
	return nil
}

// SetAllUnstablePeriods sets every unstable period (TA_FUNC_UNST_ALL).
func SetAllUnstablePeriods(period int) error {
	if period < 0 {
		return badParam("SetAllUnstablePeriods", "period", "%d is negative", period)
	}
	settings.Lock()
	for i := range settings.unstable {
		settings.unstable[i] = period
	}
	settings.Unlock()
	return nil
}

// UnstablePeriod returns the unstable period for id (TA_GetUnstablePeriod).
func UnstablePeriod(id FuncUnstID) int {
	if id < 0 || id >= unstCount {
		return 0
	}
	settings.RLock()
	defer settings.RUnlock()
	return settings.unstable[id]
}

// SetCandleSettings overrides one candle setting (TA_SetCandleSettings).
func SetCandleSettings(t CandleSettingType, rangeType RangeType, avgPeriod int, factor float64) error {
	if t < 0 || t >= candleSettingCount {
		return badParam("SetCandleSettings", "settingType", "unknown CandleSettingType %d", int(t))
	}
	if rangeType < RangeRealBody || rangeType > RangeShadows {
		return badParam("SetCandleSettings", "rangeType", "unknown RangeType %d", int(rangeType))
	}
	if avgPeriod < 0 {
		return badParam("SetCandleSettings", "avgPeriod", "%d is negative", avgPeriod)
	}
	settings.Lock()
	settings.candle[t] = CandleSetting{rangeType, avgPeriod, factor}
	settings.Unlock()
	return nil
}

// RestoreCandleDefaultSettings restores one candle setting to its default.
func RestoreCandleDefaultSettings(t CandleSettingType) {
	if t < 0 || t >= candleSettingCount {
		return
	}
	settings.Lock()
	settings.candle[t] = defaultCandleSettings[t]
	settings.Unlock()
}

// RestoreAllCandleDefaultSettings restores every candle setting.
func RestoreAllCandleDefaultSettings() {
	settings.Lock()
	settings.candle = defaultCandleSettings
	settings.Unlock()
}

// GetCandleSetting returns the current value of one candle setting.
func GetCandleSetting(t CandleSettingType) CandleSetting {
	settings.RLock()
	defer settings.RUnlock()
	return settings.candle[t]
}
