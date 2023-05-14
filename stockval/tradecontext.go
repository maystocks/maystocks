// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockval

type TradeContext struct {
	ExtendedHours bool
	UpdateLast    bool
	UpdateHighLow bool
	UpdateVolume  bool
}

type TradeConditionIndex int

// Defaults to "normal trade"
func NewTradeContext() TradeContext {
	return TradeContext{ExtendedHours: false, UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}

// Source for combining multiple trade conditions:
// https://www.nyse.com/publicdocs/ctaplan/notifications/trader-update/cts_output_spec.pdf
// page 118ff
func (a TradeContext) Combine(b TradeContext) TradeContext {
	return TradeContext{
		ExtendedHours: a.ExtendedHours || b.ExtendedHours,
		UpdateLast:    a.UpdateLast && b.UpdateLast,
		UpdateHighLow: a.UpdateHighLow && b.UpdateHighLow,
		UpdateVolume:  a.UpdateVolume && b.UpdateVolume}
}

// https://www.nyse.com/publicdocs/ctaplan/notifications/trader-update/cts_output_spec.pdf
// page 115
var TradeConditionMapCts = map[string]TradeContext{
	"@": TradeConditionRegular(),
	" ": TradeConditionRegular(),
	"B": TradeConditionAveragePrice(),
	"C": TradeConditionCashSale(),
	"E": TradeConditionAutomaticExecution(),
	"F": TradeConditionIntermarketSweepOrder(),
	"H": TradeConditionPriceVariationTrade(),
	"I": TradeConditionOddLotTrade(),
	"K": TradeConditionRule127(), // may be rule 155, but not relevant
	"L": TradeConditionSoldLast(),
	"M": TradeConditionMarketCenterOfficialClose(),
	"N": TradeConditionNextDay(),
	"O": TradeConditionMarketCenterOpeningTrade(),
	"P": TradeConditionPriorReferencePrice(),
	"Q": TradeConditionMarketCenterOfficialOpen(),
	"R": TradeConditionSeller(),
	"T": TradeConditionFormTTrade(),
	"U": TradeConditionExtendedHoursSoldOutOfSequence(),
	"V": TradeConditionContingentTrade(),
	"X": TradeConditionCrossTrade(),
	"Z": TradeConditionSoldOutOfSequence(),
	"4": TradeConditionDerivativelyPriced(),
	"5": TradeConditionMarketCenterReopeningTrade(),
	"6": TradeConditionMarketCenterClosingTrade(),
	"7": TradeConditionQualifiedContigentTrade(),
	"9": TradeConditionCorrectedConsolidatedClose(),
}

// https://www.utpplan.com/DOC/UtpBinaryOutputSpec.pdf
// page 43
var TradeConditionMapUtp = map[string]TradeContext{
	"@": TradeConditionRegular(),
	"A": TradeConditionAcquisition(),
	"B": TradeConditionBunched(),
	"C": TradeConditionCashSale(),
	"D": TradeConditionDistribution(),
	"F": TradeConditionIntermarketSweepOrder(),
	"G": TradeConditionBunchedSold(),
	"H": TradeConditionPriceVariationTrade(),
	"I": TradeConditionOddLotTrade(),
	"K": TradeConditionRule155(),
	"L": TradeConditionSoldLast(),
	"M": TradeConditionMarketCenterOfficialClose(),
	"N": TradeConditionNextDay(),
	"O": TradeConditionOpeningPrints(),
	"P": TradeConditionPriorReferencePrice(),
	"Q": TradeConditionMarketCenterOfficialOpen(),
	"R": TradeConditionSeller(),
	"S": TradeConditionSplitTrade(),
	"T": TradeConditionFormTTrade(),
	"U": TradeConditionExtendedHoursSoldOutOfSequence(),
	"V": TradeConditionContingentTrade(),
	"W": TradeConditionAveragePrice(),
	"X": TradeConditionCrossTrade(),
	"Y": TradeConditionYellowFlag(),
	"Z": TradeConditionSoldOutOfSequence(),
	"1": TradeConditionStoppedStock(),
	"4": TradeConditionDerivativelyPriced(),
	"5": TradeConditionReopeningPrints(),
	"6": TradeConditionClosingPrints(),
	"7": TradeConditionQualifiedContigentTrade(),
	"8": TradeConditionPlaceholderFor611Exempt(),
	"9": TradeConditionCorrectedConsolidatedClose(),
}

var TapeConditionMap = map[string]map[string]TradeContext{
	"A": TradeConditionMapCts,
	"B": TradeConditionMapCts,
	"C": TradeConditionMapUtp,
}

// Simplified trading conditions (special cases are not handled)
// Source: Finnhub documentation at https://docs.google.com/spreadsheets/d/1PUxiSWPHSODbaTaoL2Vef6DgU-yFtlRGZf19oBb9Hp0/edit?usp=sharing
// with #1 treated as NO, #2 treated as NO, #3 treated as YES, #4 treated as NO
// Using "market center processing" columns, as per https://alpaca.markets/learn/stock-minute-bars/
// Modified properties for Form T trades, in order to provide candles.
// Consider Form-T trades for candles outside normal market hours.
// NOTE: If values are inserted here, constants below also need to be changed.
func TradeConditionRegular() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionAcquisition() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionAveragePrice() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: true}
}
func TradeConditionBunched() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionCashSale() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: true}
}
func TradeConditionDistribution() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionAutomaticExecution() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionIntermarketSweepOrder() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionBunchedSold() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionPriceVariationTrade() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: true}
}
func TradeConditionCapElection() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: true}
}
func TradeConditionOddLotTrade() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: true}
}
func TradeConditionRule127() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionRule155() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionSoldLast() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionMarketCenterOfficialClose() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: false}
}
func TradeConditionNextDay() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: true}
}
func TradeConditionMarketCenterOpeningTrade() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionOpeningPrints() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionMarketCenterOfficialOpen() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: true, UpdateVolume: false}
}
func TradeConditionPriorReferencePrice() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionSeller() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: true}
}
func TradeConditionSplitTrade() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionFormTTrade() TradeContext {
	return TradeContext{ExtendedHours: true, UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionExtendedHoursSoldOutOfSequence() TradeContext {
	return TradeContext{ExtendedHours: true, UpdateLast: false, UpdateHighLow: false, UpdateVolume: true}
}
func TradeConditionContingentTrade() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: true}
}
func TradeConditionStockOptionTrade() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: false}
}
func TradeConditionCrossTrade() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionYellowFlag() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionSoldOutOfSequence() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionStoppedStock() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: false}
}
func TradeConditionDerivativelyPriced() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionMarketCenterReopeningTrade() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionReopeningPrints() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: false}
}
func TradeConditionMarketCenterClosingTrade() TradeContext {
	return TradeContext{UpdateLast: true, UpdateHighLow: true, UpdateVolume: true}
}
func TradeConditionClosingPrints() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: false}
}
func TradeConditionQualifiedContigentTrade() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: true}
}
func TradeConditionPlaceholderFor611Exempt() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: false}
}
func TradeConditionCorrectedConsolidatedClose() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: false}
}
func TradeConditionOpened() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: false}
}
func TradeConditionTradeThroughExempt() TradeContext {
	return TradeContext{UpdateLast: false, UpdateHighLow: false, UpdateVolume: false}
}
