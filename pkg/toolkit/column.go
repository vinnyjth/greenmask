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

package toolkit

type Column struct {
	Idx               int    `json:"idx"`
	Name              string `json:"name"`
	TypeName          string `json:"type_name"`
	CanonicalTypeName string `json:"canonical_type_name"`
	TypeOid           Oid    `json:"type_oid"`
	Num               AttNum `json:"num"`
	NotNull           bool   `json:"not_null"`
	Length            int    `json:"length"`
	// OverriddenTypeName - replacement of  original type. For instance override TEXT to INT2
	OverriddenTypeName string `json:"overridden_type_name"`
	OverriddenTypeOid  Oid    `json:"overridden_type_oid"`
}
