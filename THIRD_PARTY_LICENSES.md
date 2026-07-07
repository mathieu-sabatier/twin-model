# Third-Party Licenses

`twinmodel` is distributed under the MIT License (see [LICENSE](LICENSE)). It
bundles and depends on the third-party components listed below. All are under
permissive licenses (MIT, Apache-2.0, BSD, ISC) compatible with redistribution.

---

## Bundled OPC Foundation companion specifications

The NodeSet2 XML files under [`internal/nodeset/specs/`](internal/nodeset/specs/)
are vendored, unmodified, from the OPC Foundation
[UA-Nodeset](https://github.com/OPCFoundation/UA-Nodeset) repository. Exact
files, namespace URIs, and the upstream commit they were fetched from are
recorded in [`internal/nodeset/specs/SOURCES.md`](internal/nodeset/specs/SOURCES.md).

Covered specs: OPC UA Core (ns0), DI, IA, IRDI, Machinery, Machinery Jobs,
Machinery ProcessValues, MachineTool, PADIM, PackML, Robotics, Scales, ISA-95,
and ISA95-JobControl.

Each file carries its own license header. They are released by the OPC
Foundation under the **OPC Foundation MIT License 1.00**:

> Copyright (c) 2005-2026 The OPC Foundation, Inc. All rights reserved.
>
> OPC Foundation MIT License 1.00
>
> Permission is hereby granted, free of charge, to any person obtaining a copy
> of this software and associated documentation files (the "Software"), to deal
> in the Software without restriction, including without limitation the rights
> to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
> copies of the Software, and to permit persons to whom the Software is
> furnished to do so, subject to the following conditions:
>
> The above copyright notice and this permission notice shall be included in
> all copies or substantial portions of the Software.
>
> THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
> IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
> FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
> AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
> LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
> OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
> SOFTWARE.
>
> The complete license agreement can be found here:
> http://opcfoundation.org/License/MIT/1.00/

---

## Go dependencies

| Module | License |
|--------|---------|
| github.com/go-git/go-git/v5 | Apache-2.0 |
| github.com/go-git/go-billy/v5 | Apache-2.0 |
| gopkg.in/yaml.v3 | MIT and Apache-2.0 |

Transitive dependencies (see `go.sum`) are all under permissive licenses
(MIT, Apache-2.0, BSD-2/3-Clause, ISC).

---

## UI dependencies

The web UI (`ui/`) is built with the following principal runtime and build
dependencies (see `ui/package.json` / `ui/package-lock.json` for the full,
authoritative tree):

| Package | License |
|---------|---------|
| nuxt | MIT |
| vue, vue-router | MIT |
| @nuxt/ui | MIT |
| @pinia/nuxt, pinia | MIT |
| @vueuse/core | MIT |
| @iconify-json/lucide | MIT (icons: ISC) |
| mermaid | MIT |
| zod | MIT |
| tailwindcss | MIT |
| typescript | Apache-2.0 |
| vitest, @vue/test-utils, @nuxt/test-utils | MIT |
| happy-dom, msw | MIT |

All transitive npm dependencies resolve to permissive licenses
(MIT, Apache-2.0, BSD, ISC).
