// Copyright 2022 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package params

// APIErrorResponse holds information about an error, returned by the API
type APIErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details"`
}

var (
	// NotFoundResponse is returned when a resource is not found
	NotFoundResponse = APIErrorResponse{
		Error:   "Not Found",
		Details: "The resource you are looking for was not found",
	}
	// UnauthorizedResponse is a canned response for unauthorized access
	UnauthorizedResponse = APIErrorResponse{
		Error:   "Not Authorized",
		Details: "You do not have the required permissions to access this resource",
	}
	// InitializationRequired is returned if gopherbin has not beed properly initialized
	InitializationRequired = APIErrorResponse{
		Error:   "init_required",
		Details: "Missing superuser",
	}
	// URLsRequired is returned if the controller does not have the required URLs
	URLsRequired = APIErrorResponse{
		Error:   "urls_required",
		Details: "Missing required URLs. Make sure you update the metadata, callback and webhook URLs",
	}
)
