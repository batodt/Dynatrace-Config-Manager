// @license
// Copyright 2022 Dynatrace LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"time"
	"unicode"

	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/internal/throttle"
	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/pkg/match/rules"
	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/pkg/rest"
)

const pathEntitiesObjects = "/api/v2/entities"
const pathEntitiesTypes = "/api/v2/entityTypes"

// Full export would be: const defaultListEntitiesFields = "+lastSeenTms,+firstSeenTms,+tags,+managementZones,+toRelationships,+fromRelationships,+icon,+properties"
// Using smaller export for faster processing, using less memory
const defaultListEntitiesFields = "+lastSeenTms,+firstSeenTms"

var extraEntitiesFields = map[string][]string{
	"toRelationships": {"isSiteOf", "isClusterOfHost"},
}

func getEntitiesTypeFields(entitiesType EntitiesType, ignoreProperties []string) string {
	typeFields := defaultListEntitiesFields
	typeFields = addFields(extraEntitiesFields, ignoreProperties, entitiesType, typeFields)

	rulesFieldsL2Index := rules.GenExtraFieldsL2(&rules.INDEX_CONFIG_LIST_ENTITIES)
	typeFields = addFields(rulesFieldsL2Index, ignoreProperties, entitiesType, typeFields)

	rulesFieldsL2Hierarchy := rules.GenExtraFieldsL2(&rules.HIERARCHY_SOURCE_LIST_ENTITIES)
	typeFields = addFields(rulesFieldsL2Hierarchy, ignoreProperties, entitiesType, typeFields)

	return typeFields
}

func addFields(extraEntitiesFields map[string][]string, ignoreProperties []string, entitiesType EntitiesType, typeFields string) string {

	for topField, subFieldList := range extraEntitiesFields {

		if contains(ignoreProperties, topField) {
			continue
		}

		for _, subField := range subFieldList {
			if contains(ignoreProperties, subField) {
				continue
			}

			_, exists := L2FieldWithIdExists(entitiesType, topField, subField)

			if exists {
				typeFields = typeFields + ",+" + topField + "." + subField
			}
		}
	}

	return typeFields
}

func L2FieldWithIdExists(entitiesType EntitiesType, topField string, subField string) (reflect.Value, bool) {

	fieldSliceObject := GetDynamicFieldFromObject(entitiesType, topField)

	if IsInvalidReflectionValue(fieldSliceObject) {
		return reflect.Value{}, false
	}

	return hasSpecificFieldValueInSlice(fieldSliceObject, "id", subField)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func GetDynamicFieldFromObject(object interface{}, field string) reflect.Value {
	reflection := reflect.ValueOf(object)

	fieldValue := reflect.Indirect(reflection).FieldByName(field) // nosemgrep: go.lang.security.audit.unsafe-reflect-by-name.unsafe-reflect-by-name

	// We are providing uncapitalized fields from json maps
	// But GoLang forces capitalized for unmarshalling
	// Let's try a capitalized first letter
	if IsInvalidReflectionValue(fieldValue) {
		field = capitalizeFirstLetter(field)
		fieldValue = reflect.Indirect(reflection).FieldByName(field) // nosemgrep: go.lang.security.audit.unsafe-reflect-by-name.unsafe-reflect-by-name
	}
	return fieldValue
}

func GetDynamicFieldFromMapReflection(reflection reflect.Value, field string) reflect.Value {
	return reflection.MapIndex(reflect.ValueOf(field))
}

func hasSpecificFieldValueInSlice(slice reflect.Value, field string, searchFieldValue string) (reflect.Value, bool) {

	for i := 0; i < slice.Len(); i++ {
		element := slice.Index(i)
		if IsInvalidReflectionValue(element) {
			continue
		}

		idValueString := GetStringFromMapReflection(element, field)

		if idValueString == searchFieldValue {
			return element, true
		}
	}

	return reflect.Value{}, false

}

func GetStringFromMapReflection(element reflect.Value, field string) string {
	idValue := GetDynamicFieldFromMapReflection(element, field)

	if IsInvalidReflectionValue(idValue) {
		return ""
	}

	stringValue, isString := idValue.Interface().(string)

	if isString {
		return stringValue
	}

	return ""
}

func capitalizeFirstLetter(str string) string {
	runes := []rune(str)
	runes[0] = unicode.ToUpper(runes[0])
	capitalizedString := string(runes)

	return capitalizedString
}

func IsInvalidReflectionValue(value reflect.Value) bool {
	if value.Kind() == reflect.Invalid {
		return true
	} else {
		return false
	}
}

func genListEntitiesParams(entityType string, entitiesType EntitiesType, opts ListEntitiesOptions, ignoreProperties []string) (url.Values, string, string) {
	from := genTimeframeUnixMilliString(-1 * time.Duration(opts.TimeFromMinutes) * time.Minute)
	to := genTimeframeUnixMilliString(-1 * time.Duration(opts.TimeToMinutes) * time.Minute)

	pageSize := DefaultPageSizeEntities
	if opts.EntityPageSize > 0 {
		pageSize = fmt.Sprintf("%d", opts.EntityPageSize)
	}

	params := url.Values{
		"entitySelector": []string{"type(\"" + entityType + "\")"},
		"pageSize":       []string{pageSize},
		"fields":         []string{getEntitiesTypeFields(entitiesType, ignoreProperties)},
		"from":           []string{from},
		"to":             []string{to},
	}

	return params, from, to
}

func handleListEntitiesError(entityType string, resp rest.Response, run_extraction bool, ignoreProperties []string, err error) (bool, []string, error) {
	if err != nil {
		retryWithIgnore := false
		retryWithIgnore, ignoreProperties = validateForPropertyErrors(resp, ignoreProperties, entityType)

		if retryWithIgnore {
			return run_extraction, ignoreProperties, nil
		} else {
			return run_extraction, ignoreProperties, err
		}
	} else {
		return false, ignoreProperties, nil
	}
}

type ErrorResponseStruct struct {
	ErrorResponse ErrorResponse `json:"error"`
}

type ErrorResponse struct {
	ErrorCode               int                   `json:"code"`
	Message                 string                `json:"message"`
	ConstraintViolationList []ConstraintViolation `json:"constraintViolations"`
}

type ConstraintViolation struct {
	Path              string `json:"path"`
	Message           string `json:"message"`
	ParameterLocation string `json:"parameterLocation"`
	Location          string `json:"location"`
}

// errorPropertyNameRegexPattern extract from error text
// capturing the property name between single quotes
// Sample format: "message": "'test' is not a valid property for type 'SOFTWARE_COMPONENT'"
var errorPropertyNameRegexPattern = regexp.MustCompile(`'([^']+)'.*`)

func validateForPropertyErrors(resp rest.Response, ignoreProperties []string, entityType string) (bool, []string) {
	retryWithIgnore := false

	var errorResponse ErrorResponseStruct
	err := json.Unmarshal(resp.Body, &errorResponse)

	if err == nil {
		if errorResponse.ErrorResponse.ErrorCode == 400 {
			constraintViolationList := errorResponse.ErrorResponse.ConstraintViolationList
			for _, constraintViolation := range constraintViolationList {
				if constraintViolation.Path == "fields" {
					matches := errorPropertyNameRegexPattern.FindStringSubmatch(constraintViolation.Message)
					if len(matches) >= 2 {
						if contains(ignoreProperties, matches[1]) {
							continue
						}
						ignoreProperties = append(ignoreProperties, matches[1])
						throttle.ThrottleCallAfterError(1, "Property error in type: %s: will not extract: %s", entityType, matches[1])
						retryWithIgnore = true
					}
				}
			}
		}
	}

	return retryWithIgnore, ignoreProperties
}
