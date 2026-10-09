// Copyright 2026 Cloudbase Solutions SRL
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

package cmd

import (
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// paginatedListHeader builds the merged table header row that paginated
// listings place above their column headers.
func paginatedListHeader(numCols int, totalCount, pages, currentPage uint64) (table.Row, table.RowConfig) {
	// An empty listing reports zero pages, display it as one.
	if pages == 0 {
		pages = 1
	}
	headerText := fmt.Sprintf("Page %d of %d (%d results)", currentPage, pages, totalCount)
	header := make(table.Row, numCols)
	for i := range header {
		header[i] = headerText
	}
	return header, table.RowConfig{
		AutoMerge:      true,
		AutoMergeAlign: text.AlignCenter,
	}
}
