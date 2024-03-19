/*
Copyright (c) 2020 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// IMPORTANT: This file has been generated automatically, refrain from modifying it manually as all
// your changes will be lost when the file is generated again.

package v1 // github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1

import (
	"io"

	jsoniter "github.com/json-iterator/go"
	"github.com/openshift-online/ocm-sdk-go/helpers"
)

// MarshalValue writes a value of the 'value' type to the given writer.
func MarshalValue(object *Value, writer io.Writer) error {
	stream := helpers.NewStream(writer)
	writeValue(object, stream)
	err := stream.Flush()
	if err != nil {
		return err
	}
	return stream.Error
}

// writeValue writes a value of the 'value' type to the given stream.
func writeValue(object *Value, stream *jsoniter.Stream) {
	count := 0
	stream.WriteObjectStart()
	var present_ bool
	present_ = object.bitmap_&1 != 0
	if present_ {
		if count > 0 {
			stream.WriteMore()
		}
		stream.WriteObjectField("unit")
		stream.WriteString(object.unit)
		count++
	}
	present_ = object.bitmap_&2 != 0
	if present_ {
		if count > 0 {
			stream.WriteMore()
		}
		stream.WriteObjectField("value")
		stream.WriteFloat64(object.value)
	}
	stream.WriteObjectEnd()
}

// UnmarshalValue reads a value of the 'value' type from the given
// source, which can be an slice of bytes, a string or a reader.
func UnmarshalValue(source interface{}) (object *Value, err error) {
	iterator, err := helpers.NewIterator(source)
	if err != nil {
		return
	}
	object = readValue(iterator)
	err = iterator.Error
	return
}

// readValue reads a value of the 'value' type from the given iterator.
func readValue(iterator *jsoniter.Iterator) *Value {
	object := &Value{}
	for {
		field := iterator.ReadObject()
		if field == "" {
			break
		}
		switch field {
		case "unit":
			value := iterator.ReadString()
			object.unit = value
			object.bitmap_ |= 1
		case "value":
			value := iterator.ReadFloat64()
			object.value = value
			object.bitmap_ |= 2
		default:
			iterator.ReadAny()
		}
	}
	return object
}
