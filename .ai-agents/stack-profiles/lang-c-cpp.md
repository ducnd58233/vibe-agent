# Stack profile: Language C / C++

## Scope

Applies to consumer repositories writing C or C++ at the language and toolchain level: memory and object lifetime, undefined behavior, build systems, sanitizers, ABI and FFI boundaries. Independent of any application framework; compose with a domain profile when one applies.

## When to load

- Writing or reviewing C/C++ translation units, headers, or templates
- Diagnosing crashes, leaks, data races, or undefined behavior
- Build-system, cross-compilation, or dependency changes
- Designing a C ABI for FFI to another language
- Choosing or raising the language standard for a target

## Detection

- `CMakeLists.txt`, `Makefile`, `meson.build`, `BUILD.bazel`, `configure.ac`
- Sources: `*.c`, `*.h`, `*.cc`, `*.cpp`, `*.cxx`, `*.hpp`, `*.ipp`
- `compile_commands.json`, `.clang-tidy`, `.clang-format`
- Dependency manifests such as `conanfile.txt`/`conanfile.py`, `vcpkg.json`

## Framework and tooling

Non-exhaustive examples. Any tool here may be renamed, deprecated, or replaced. Detect what the repo actually uses from its manifests and CI, and verify current commands and flags against official docs ([`source-driven-development`](../skills/source-driven-development/SKILL.md)) before running anything.

- Compilers: for example GCC, Clang, MSVC — flags and warning sets differ per compiler and version
- Build and configure: for example CMake, Meson, Bazel, plain Make
- Dependencies: for example Conan, vcpkg, system packages, vendored submodules
- Sanitizers: for example AddressSanitizer, UndefinedBehaviorSanitizer, ThreadSanitizer, MemorySanitizer
- Static analysis and format: for example clang-tidy, cppcheck, clang-format, include-what-you-use
- Debug and profile: for example gdb, lldb, Valgrind, perf
- Fuzzing and property tests: for example libFuzzer, AFL++

## Repo layout conventions

- Read the build files and `README.md` before source; the build graph defines what actually compiles
- Common split: public headers under `include/`, implementation under `src/`, tests under `tests/`
- Generate or locate `compile_commands.json` so analysis tools see real flags
- Keep the language standard explicit in the build files, not implied by compiler defaults

## Commands

Use repo-documented commands first. Typical examples:

- Configure and build: `cmake -S . -B build && cmake --build build`
- Test: `ctest --test-dir build --output-on-failure`
- Sanitizer build: configure a separate build directory with the sanitizer flags the repo defines
- Static analysis: `clang-tidy` driven by `compile_commands.json`

## Boundaries

- Treat undefined behavior as a correctness bug, not a style issue: signed overflow, out-of-bounds access, use-after-free, strict-aliasing violations, uninitialized reads, data races
- C++: prefer RAII and ownership types over manual `new`/`delete`; follow the rule of zero, and rule of three/five only when a type genuinely manages a resource
- Do not pass ownership of memory across an ABI boundary allocated by a different allocator or runtime
- Keep FFI surfaces to a narrow, documented C ABI; C++ symbols are not stable across compilers or versions
- Do not add a dependency without checking how the repo's build system consumes dependencies
- Do not silence a sanitizer or warning to make a build pass; fix the cause or record an explicit, justified suppression

## Security / performance appendix

- Never use unbounded string or buffer operations on untrusted input; prefer sized APIs the repo already uses
- Validate integer conversions and container indices at trust boundaries
- Measure before optimizing; compiler optimization level and build type change results substantially
- Run the test suite under sanitizers in CI where the project supports it — sanitizer findings are defects even when tests pass

## References

- https://en.cppreference.com/w/
- https://isocpp.github.io/CppCoreGuidelines/CppCoreGuidelines
- https://clang.llvm.org/docs/index.html
- https://cmake.org/cmake/help/latest/
