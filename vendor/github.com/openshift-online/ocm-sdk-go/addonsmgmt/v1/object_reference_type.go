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

package v1 // github.com/openshift-online/ocm-sdk-go/addonsmgmt/v1

// ObjectReference represents the values of the 'object_reference' type.
//
// representation of object reference/subscription
type ObjectReference struct {
	bitmap_ uint32
	href    string
	id      string
	kind    string
}

// Empty returns true if the object is empty, i.e. no attribute has a value.
func (o *ObjectReference) Empty() bool {
	return o == nil || o.bitmap_ == 0
}

// Href returns the value of the 'href' attribute, or
// the zero value of the type if the attribute doesn't have a value.
//
// Self Link
func (o *ObjectReference) Href() string {
	if o != nil && o.bitmap_&1 != 0 {
		return o.href
	}
	return ""
}

// GetHref returns the value of the 'href' attribute and
// a flag indicating if the attribute has a value.
//
// Self Link
func (o *ObjectReference) GetHref() (value string, ok bool) {
	ok = o != nil && o.bitmap_&1 != 0
	if ok {
		value = o.href
	}
	return
}

// Id returns the value of the 'id' attribute, or
// the zero value of the type if the attribute doesn't have a value.
//
// Unique identifier of the object.
func (o *ObjectReference) Id() string {
	if o != nil && o.bitmap_&2 != 0 {
		return o.id
	}
	return ""
}

// GetId returns the value of the 'id' attribute and
// a flag indicating if the attribute has a value.
//
// Unique identifier of the object.
func (o *ObjectReference) GetId() (value string, ok bool) {
	ok = o != nil && o.bitmap_&2 != 0
	if ok {
		value = o.id
	}
	return
}

// Kind returns the value of the 'kind' attribute, or
// the zero value of the type if the attribute doesn't have a value.
//
// Indicates the type of this object.
func (o *ObjectReference) Kind() string {
	if o != nil && o.bitmap_&4 != 0 {
		return o.kind
	}
	return ""
}

// GetKind returns the value of the 'kind' attribute and
// a flag indicating if the attribute has a value.
//
// Indicates the type of this object.
func (o *ObjectReference) GetKind() (value string, ok bool) {
	ok = o != nil && o.bitmap_&4 != 0
	if ok {
		value = o.kind
	}
	return
}

// ObjectReferenceListKind is the name of the type used to represent list of objects of
// type 'object_reference'.
const ObjectReferenceListKind = "ObjectReferenceList"

// ObjectReferenceListLinkKind is the name of the type used to represent links to list
// of objects of type 'object_reference'.
const ObjectReferenceListLinkKind = "ObjectReferenceListLink"

// ObjectReferenceNilKind is the name of the type used to nil lists of objects of
// type 'object_reference'.
const ObjectReferenceListNilKind = "ObjectReferenceListNil"

// ObjectReferenceList is a list of values of the 'object_reference' type.
type ObjectReferenceList struct {
	href  string
	link  bool
	items []*ObjectReference
}

// Len returns the length of the list.
func (l *ObjectReferenceList) Len() int {
	if l == nil {
		return 0
	}
	return len(l.items)
}

// Empty returns true if the list is empty.
func (l *ObjectReferenceList) Empty() bool {
	return l == nil || len(l.items) == 0
}

// Get returns the item of the list with the given index. If there is no item with
// that index it returns nil.
func (l *ObjectReferenceList) Get(i int) *ObjectReference {
	if l == nil || i < 0 || i >= len(l.items) {
		return nil
	}
	return l.items[i]
}

// Slice returns an slice containing the items of the list. The returned slice is a
// copy of the one used internally, so it can be modified without affecting the
// internal representation.
//
// If you don't need to modify the returned slice consider using the Each or Range
// functions, as they don't need to allocate a new slice.
func (l *ObjectReferenceList) Slice() []*ObjectReference {
	var slice []*ObjectReference
	if l == nil {
		slice = make([]*ObjectReference, 0)
	} else {
		slice = make([]*ObjectReference, len(l.items))
		copy(slice, l.items)
	}
	return slice
}

// Each runs the given function for each item of the list, in order. If the function
// returns false the iteration stops, otherwise it continues till all the elements
// of the list have been processed.
func (l *ObjectReferenceList) Each(f func(item *ObjectReference) bool) {
	if l == nil {
		return
	}
	for _, item := range l.items {
		if !f(item) {
			break
		}
	}
}

// Range runs the given function for each index and item of the list, in order. If
// the function returns false the iteration stops, otherwise it continues till all
// the elements of the list have been processed.
func (l *ObjectReferenceList) Range(f func(index int, item *ObjectReference) bool) {
	if l == nil {
		return
	}
	for index, item := range l.items {
		if !f(index, item) {
			break
		}
	}
}
