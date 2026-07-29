/*
Copyright 2025.

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

// Command cmdref is the command reference generator invoked by the shared build
// tasks (hack/common) via the REFERENCE_GENERATOR variable, which defaults to
// this path. It receives the output directory as its single argument.
//
// This repository is a controller / service provider, not a cobra CLI, so there
// is no command tree to document. The generator is a deliberate no-op that exists
// only so the "generate:command-reference" task succeeds instead of failing on a
// missing file.
package main

func main() {
	// No CLI command reference to generate for this repository.
}
