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
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/openshift-online/ocm-sdk-go/errors"
	"github.com/openshift-online/ocm-sdk-go/helpers"
)

// GCPRegionMachineTypesInquiryClient is the client of the 'GCP_region_machine_types_inquiry' resource.
//
// Manages machine types in region inquiry.
type GCPRegionMachineTypesInquiryClient struct {
	transport http.RoundTripper
	path      string
}

// NewGCPRegionMachineTypesInquiryClient creates a new client for the 'GCP_region_machine_types_inquiry'
// resource using the given transport to send the requests and receive the
// responses.
func NewGCPRegionMachineTypesInquiryClient(transport http.RoundTripper, path string) *GCPRegionMachineTypesInquiryClient {
	return &GCPRegionMachineTypesInquiryClient{
		transport: transport,
		path:      path,
	}
}

// Search creates a request for the 'search' method.
//
// Retrieves the list of machine types in the provided region.
func (c *GCPRegionMachineTypesInquiryClient) Search() *GCPRegionMachineTypesInquirySearchRequest {
	return &GCPRegionMachineTypesInquirySearchRequest{
		transport: c.transport,
		path:      c.path,
	}
}

// GCPRegionMachineTypesInquirySearchRequest is the request for the 'search' method.
type GCPRegionMachineTypesInquirySearchRequest struct {
	transport http.RoundTripper
	path      string
	query     url.Values
	header    http.Header
	body      *CloudProviderData
	page      *int
	size      *int
}

// Parameter adds a query parameter.
func (r *GCPRegionMachineTypesInquirySearchRequest) Parameter(name string, value interface{}) *GCPRegionMachineTypesInquirySearchRequest {
	helpers.AddValue(&r.query, name, value)
	return r
}

// Header adds a request header.
func (r *GCPRegionMachineTypesInquirySearchRequest) Header(name string, value interface{}) *GCPRegionMachineTypesInquirySearchRequest {
	helpers.AddHeader(&r.header, name, value)
	return r
}

// Impersonate wraps requests on behalf of another user.
// Note: Services that do not support this feature may silently ignore this call.
func (r *GCPRegionMachineTypesInquirySearchRequest) Impersonate(user string) *GCPRegionMachineTypesInquirySearchRequest {
	helpers.AddImpersonationHeader(&r.header, user)
	return r
}

// Body sets the value of the 'body' parameter.
//
// Cloud provider data needed for the inquiry
func (r *GCPRegionMachineTypesInquirySearchRequest) Body(value *CloudProviderData) *GCPRegionMachineTypesInquirySearchRequest {
	r.body = value
	return r
}

// Page sets the value of the 'page' parameter.
//
// Index of the requested page, where one corresponds to the first page.
func (r *GCPRegionMachineTypesInquirySearchRequest) Page(value int) *GCPRegionMachineTypesInquirySearchRequest {
	r.page = &value
	return r
}

// Size sets the value of the 'size' parameter.
//
// Maximum number of items that will be contained in the returned page.
func (r *GCPRegionMachineTypesInquirySearchRequest) Size(value int) *GCPRegionMachineTypesInquirySearchRequest {
	r.size = &value
	return r
}

// Send sends this request, waits for the response, and returns it.
//
// This is a potentially lengthy operation, as it requires network communication.
// Consider using a context and the SendContext method.
func (r *GCPRegionMachineTypesInquirySearchRequest) Send() (result *GCPRegionMachineTypesInquirySearchResponse, err error) {
	return r.SendContext(context.Background())
}

// SendContext sends this request, waits for the response, and returns it.
func (r *GCPRegionMachineTypesInquirySearchRequest) SendContext(ctx context.Context) (result *GCPRegionMachineTypesInquirySearchResponse, err error) {
	query := helpers.CopyQuery(r.query)
	if r.page != nil {
		helpers.AddValue(&query, "page", *r.page)
	}
	if r.size != nil {
		helpers.AddValue(&query, "size", *r.size)
	}
	header := helpers.CopyHeader(r.header)
	buffer := &bytes.Buffer{}
	err = writeGCPRegionMachineTypesInquirySearchRequest(r, buffer)
	if err != nil {
		return
	}
	uri := &url.URL{
		Path:     r.path,
		RawQuery: query.Encode(),
	}
	request := &http.Request{
		Method: "POST",
		URL:    uri,
		Header: header,
		Body:   io.NopCloser(buffer),
	}
	if ctx != nil {
		request = request.WithContext(ctx)
	}
	response, err := r.transport.RoundTrip(request)
	if err != nil {
		return
	}
	defer response.Body.Close()
	result = &GCPRegionMachineTypesInquirySearchResponse{}
	result.status = response.StatusCode
	result.header = response.Header
	reader := bufio.NewReader(response.Body)
	_, err = reader.Peek(1)
	if err == io.EOF {
		err = nil
		return
	}
	if result.status >= 400 {
		result.err, err = errors.UnmarshalErrorStatus(reader, result.status)
		if err != nil {
			return
		}
		err = result.err
		return
	}
	err = readGCPRegionMachineTypesInquirySearchResponse(result, reader)
	if err != nil {
		return
	}
	return
}

// GCPRegionMachineTypesInquirySearchResponse is the response for the 'search' method.
type GCPRegionMachineTypesInquirySearchResponse struct {
	status int
	header http.Header
	err    *errors.Error
	items  *MachineTypeList
	page   *int
	size   *int
	total  *int
}

// Status returns the response status code.
func (r *GCPRegionMachineTypesInquirySearchResponse) Status() int {
	if r == nil {
		return 0
	}
	return r.status
}

// Header returns header of the response.
func (r *GCPRegionMachineTypesInquirySearchResponse) Header() http.Header {
	if r == nil {
		return nil
	}
	return r.header
}

// Error returns the response error.
func (r *GCPRegionMachineTypesInquirySearchResponse) Error() *errors.Error {
	if r == nil {
		return nil
	}
	return r.err
}

// Items returns the value of the 'items' parameter.
//
// Retrieved list of machine types.
func (r *GCPRegionMachineTypesInquirySearchResponse) Items() *MachineTypeList {
	if r == nil {
		return nil
	}
	return r.items
}

// GetItems returns the value of the 'items' parameter and
// a flag indicating if the parameter has a value.
//
// Retrieved list of machine types.
func (r *GCPRegionMachineTypesInquirySearchResponse) GetItems() (value *MachineTypeList, ok bool) {
	ok = r != nil && r.items != nil
	if ok {
		value = r.items
	}
	return
}

// Page returns the value of the 'page' parameter.
//
// Index of the requested page, where one corresponds to the first page.
func (r *GCPRegionMachineTypesInquirySearchResponse) Page() int {
	if r != nil && r.page != nil {
		return *r.page
	}
	return 0
}

// GetPage returns the value of the 'page' parameter and
// a flag indicating if the parameter has a value.
//
// Index of the requested page, where one corresponds to the first page.
func (r *GCPRegionMachineTypesInquirySearchResponse) GetPage() (value int, ok bool) {
	ok = r != nil && r.page != nil
	if ok {
		value = *r.page
	}
	return
}

// Size returns the value of the 'size' parameter.
//
// Maximum number of items that will be contained in the returned page.
func (r *GCPRegionMachineTypesInquirySearchResponse) Size() int {
	if r != nil && r.size != nil {
		return *r.size
	}
	return 0
}

// GetSize returns the value of the 'size' parameter and
// a flag indicating if the parameter has a value.
//
// Maximum number of items that will be contained in the returned page.
func (r *GCPRegionMachineTypesInquirySearchResponse) GetSize() (value int, ok bool) {
	ok = r != nil && r.size != nil
	if ok {
		value = *r.size
	}
	return
}

// Total returns the value of the 'total' parameter.
//
// Total number of items of the collection that match the search criteria,
// regardless of the size of the page.
func (r *GCPRegionMachineTypesInquirySearchResponse) Total() int {
	if r != nil && r.total != nil {
		return *r.total
	}
	return 0
}

// GetTotal returns the value of the 'total' parameter and
// a flag indicating if the parameter has a value.
//
// Total number of items of the collection that match the search criteria,
// regardless of the size of the page.
func (r *GCPRegionMachineTypesInquirySearchResponse) GetTotal() (value int, ok bool) {
	ok = r != nil && r.total != nil
	if ok {
		value = *r.total
	}
	return
}
