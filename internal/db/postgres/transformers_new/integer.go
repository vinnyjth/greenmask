// Copyright 2023 Greenmask
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transformers_new

import (
	"context"
	"fmt"
	"math"

	"github.com/greenmaskio/greenmask/internal/db/postgres/transformers/utils"
	"github.com/greenmaskio/greenmask/internal/generators"
	"github.com/greenmaskio/greenmask/internal/generators/transformers"
	"github.com/greenmaskio/greenmask/pkg/toolkit"
)

const (
	Int2Length = 2
	Int4Length = 4
	Int8Length = 8
)

const (
	intTransformerName        = "Integer"
	intTransformerDescription = "Generate integer value in min and max thresholds"
)

const integerTransformerGenByteLength = 8

var integerTransformerParams = []*toolkit.ParameterDefinition{
	toolkit.MustNewParameterDefinition(
		"column",
		"column name",
	).SetIsColumn(
		toolkit.NewColumnProperties().
			SetAffected(true).
			SetAllowedColumnTypes("int2", "int4", "int8"),
	).SetRequired(true),

	toolkit.MustNewParameterDefinition(
		"min",
		"min int value threshold",
	).SetLinkParameter("column").
		SetDynamicMode(
			toolkit.NewDynamicModeProperties().
				SetCompatibleTypes("int2", "int4", "int8"),
		),

	toolkit.MustNewParameterDefinition(
		"max",
		"max int value threshold",
	).SetLinkParameter("column").
		SetDynamicMode(
			toolkit.NewDynamicModeProperties().
				SetCompatibleTypes("int2", "int4", "int8"),
		),

	toolkit.MustNewParameterDefinition(
		"keep_null",
		"indicates that NULL values must not be replaced with transformed values",
	).SetDefaultValue(toolkit.ParamsValue("true")),
}

type IntegerTransformer struct {
	columnName      string
	keepNull        bool
	affectedColumns map[int]string
	columnIdx       int
	t               transformers.Transformer
	dynamicMode     bool
	intSize         int

	columnParam   toolkit.Parameterizer
	maxParam      toolkit.Parameterizer
	minParam      toolkit.Parameterizer
	keepNullParam toolkit.Parameterizer
}

func NewIntegerTransformer(ctx context.Context, driver *toolkit.Driver, parameters map[string]toolkit.Parameterizer, g generators.Generator) (utils.Transformer, toolkit.ValidationWarnings, error) {

	var columnName string
	var minVal, maxVal int64
	var keepNull, dynamicMode bool
	var intSize = 8

	columnParam := parameters["column"]
	minParam := parameters["min"]
	maxParam := parameters["max"]
	keepNullParam := parameters["keep_null"]

	if minParam.IsDynamic() || maxParam.IsDynamic() {
		dynamicMode = true
	}

	if err := columnParam.Scan(&columnName); err != nil {
		return nil, nil, fmt.Errorf(`unable to scan "column" param: %w`, err)
	}

	idx, c, ok := driver.GetColumnByName(columnName)
	if !ok {
		return nil, nil, fmt.Errorf("column with name %s is not found", columnName)
	}
	affectedColumns := make(map[int]string)
	affectedColumns[idx] = columnName
	if c.Length != -1 {
		intSize = c.Length
	}

	if err := keepNullParam.Scan(&keepNull); err != nil {
		return nil, nil, fmt.Errorf(`unable to scan "keep_null" param: %w`, err)
	}

	if !dynamicMode {
		if err := minParam.Scan(&minVal); err != nil {
			return nil, nil, fmt.Errorf("error scanning \"min\" parameter: %w", err)
		}
		if err := maxParam.Scan(&maxVal); err != nil {
			return nil, nil, fmt.Errorf("error scanning \"max\" parameter: %w", err)
		}
	}

	limiter, limitsWarnings, err := validateIntTypeAndSetLimit(intSize, minVal, maxVal)
	if err != nil {
		return nil, nil, err
	}
	if limitsWarnings.IsFatal() {
		return nil, limitsWarnings, nil
	}

	t, err := transformers.NewInt64Transformer(g, limiter)
	if err != nil {
		return nil, nil, fmt.Errorf("error initializing common int transformer: %w", err)
	}

	return &IntegerTransformer{
		columnName:      columnName,
		keepNull:        keepNull,
		affectedColumns: affectedColumns,
		columnIdx:       idx,

		columnParam:   columnParam,
		minParam:      minParam,
		maxParam:      maxParam,
		keepNullParam: keepNullParam,
		t:             t,

		dynamicMode: dynamicMode,
		intSize:     intSize,
	}, nil, nil
}

func (rit *IntegerTransformer) GetAffectedColumns() map[int]string {
	return rit.affectedColumns
}

func (rit *IntegerTransformer) Init(ctx context.Context) error {
	return nil
}

func (rit *IntegerTransformer) Done(ctx context.Context) error {
	return nil
}

func (rit *IntegerTransformer) dynamicTransform(ctx context.Context, r *toolkit.Record) (*toolkit.Record, error) {
	val, err := r.GetRawColumnValueByIdx(rit.columnIdx)
	if err != nil {
		return nil, fmt.Errorf("unable to scan value: %w", err)
	}
	if val.IsNull && rit.keepNull {
		return r, nil
	}

	var minVal, maxVal int64
	err = rit.minParam.Scan(&minVal)
	if err != nil {
		return nil, fmt.Errorf(`unable to scan "min" param: %w`, err)
	}

	err = rit.maxParam.Scan(&maxVal)
	if err != nil {
		return nil, fmt.Errorf(`unable to scan "max" param: %w`, err)
	}

	limiter, err := getLimiterForDynamicParameter(rit.intSize, minVal, maxVal)
	if err != nil {
		return nil, fmt.Errorf("error creating limiter in dynamic mode: %w", err)
	}
	ctx = context.WithValue(ctx, "limiter", limiter)
	res, err := rit.t.Transform(ctx, val.Data)
	if err != nil {
		return nil, fmt.Errorf("error generating int value: %w", err)
	}

	if err := r.SetRawColumnValueByIdx(rit.columnIdx, toolkit.NewRawValue(res, false)); err != nil {
		return nil, fmt.Errorf("unable to set new value: %w", err)
	}
	return r, nil
}

func (rit *IntegerTransformer) staticTransform(ctx context.Context, r *toolkit.Record) (*toolkit.Record, error) {
	val, err := r.GetRawColumnValueByIdx(rit.columnIdx)
	if err != nil {
		return nil, fmt.Errorf("unable to scan value: %w", err)
	}
	if val.IsNull && rit.keepNull {
		return r, nil
	}
	res, err := rit.t.Transform(ctx, val.Data)
	if err != nil {
		return nil, fmt.Errorf("error generating int value: %w", err)
	}

	if err := r.SetRawColumnValueByIdx(rit.columnIdx, toolkit.NewRawValue(res, false)); err != nil {
		return nil, fmt.Errorf("unable to set new value: %w", err)
	}
	return r, nil
}

func (rit *IntegerTransformer) Transform(ctx context.Context, r *toolkit.Record) (*toolkit.Record, error) {
	if rit.dynamicMode {
		return rit.dynamicTransform(ctx, r)
	}
	return rit.staticTransform(ctx, r)
}

func getIntThresholds(size int) (int64, int64, error) {
	switch size {
	case Int2Length:
		return math.MinInt16, math.MaxInt16, nil
	case Int4Length:
		return math.MinInt32, math.MaxInt32, nil
	case Int8Length:
		return math.MinInt16, math.MaxInt16, nil
	}

	return 0, 0, fmt.Errorf("unsupported int size %d", size)
}

func getLimiterForDynamicParameter(size int, requestedMinValue, requestedMaxValue int64) (*transformers.Int64Limiter, error) {
	minValue, maxValue, err := getIntThresholds(size)
	if err != nil {
		return nil, err
	}

	if !limitIsValid(requestedMinValue, minValue, maxValue) {
		return nil, fmt.Errorf("requested dynamic parameter min value is out of range of int%d size", size)
	}

	if !limitIsValid(requestedMaxValue, minValue, maxValue) {
		return nil, fmt.Errorf("requested dynamic parameter max value is out of range of int%d size", size)
	}

	limiter, err := transformers.NewInt64Limiter(math.MinInt64, math.MaxInt64)
	if err != nil {
		return nil, err
	}

	if requestedMinValue != 0 || requestedMaxValue != 0 {
		limiter, err = transformers.NewInt64Limiter(requestedMinValue, requestedMaxValue)
		if err != nil {
			return nil, err
		}
	}
	return limiter, nil
}

func limitIsValid(requestedThreshold, minValue, maxValue int64) bool {
	return requestedThreshold >= minValue || requestedThreshold <= maxValue
}

func validateIntTypeAndSetLimit(
	size int, requestedMinValue, requestedMaxValue int64,
) (limiter *transformers.Int64Limiter, warns toolkit.ValidationWarnings, err error) {

	minValue, maxValue, err := getIntThresholds(size)
	if err != nil {
		return nil, nil, err
	}

	if !limitIsValid(requestedMinValue, minValue, maxValue) {
		warns = append(warns, toolkit.NewValidationWarning().
			SetMsgf("requested min value is out of int%d range", size).
			SetSeverity(toolkit.ErrorValidationSeverity).
			AddMeta("AllowedMinValue", minValue).
			AddMeta("AllowedMaxValue", maxValue).
			AddMeta("ParameterName", "min").
			AddMeta("ParameterValue", requestedMinValue),
		)
	}

	if !limitIsValid(requestedMaxValue, minValue, maxValue) {
		warns = append(warns, toolkit.NewValidationWarning().
			SetMsgf("requested max value is out of int%d range", size).
			SetSeverity(toolkit.ErrorValidationSeverity).
			AddMeta("AllowedMinValue", minValue).
			AddMeta("AllowedMaxValue", maxValue).
			AddMeta("ParameterName", "min").
			AddMeta("ParameterValue", requestedMinValue),
		)
	}

	if warns.IsFatal() {
		return nil, warns, nil
	}

	limiter, err = transformers.NewInt64Limiter(math.MinInt64, math.MaxInt64)
	if err != nil {
		return nil, nil, err
	}

	if requestedMinValue != 0 || requestedMaxValue != 0 {
		limiter, err = transformers.NewInt64Limiter(requestedMinValue, requestedMaxValue)
		if err != nil {
			return nil, nil, err
		}
	}

	return limiter, nil, nil
}

func init() {

	registerRandomAndDeterministicTransformer(
		utils.DefaultTransformerRegistry,
		intTransformerName,
		intTransformerDescription,
		NewIntegerTransformer,
		integerTransformerParams,
		integerTransformerGenByteLength,
	)
}
