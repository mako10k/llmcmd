# Tool Layer Simplification Implementation Plan

## Current State Analysis

### ✅ Already Implemented
1. **VFS Integration**: `executeOpen()` already uses VFS exclusively
2. **FD Management**: Basic FD allocation and tracking in place  
3. **Shell Execution**: `executeSpawn()` with shell executor integration

### 🔄 Needs Simplification

#### 1. Remove VFS/RealFS Switching Logic
- `executeOpen()` already pure VFS - ✅ DONE
- Remove any remaining real filesystem fallbacks

#### 2. Simplify Pipeline FD Management  
- Current: Complex tracking of child process FDs
- Target: Parent closes handed-off FDs immediately after spawn
- Location: `executeSpawn()` function

#### 3. VFS Pre-population Enhancement
- Ensure CLI -i/-o files are registered in VFS at engine startup
- Location: Engine initialization in `app.go`

## Implementation Steps

### Phase 1: Pipeline FD Simplification ✅ COMPLETED
1. ✅ Modified `executeSpawn()` to close parent FDs immediately after child spawn
2. ✅ Removed complex FD chain tracking logic  
3. ✅ Simplified `executeClose()` to basic FD close operation
4. ✅ Removed complex chain tracking functions:
   - `addFdDependency()`
   - `markFdClosed()`  
   - `traverseChainOnEOF()`
   - `traverseChainRecursive()`
5. ✅ Removed complex chain tracking fields from Engine struct:
   - `fdDependencies []FdDependency`
   - `closedFds map[int]bool`
   - `chainMutex sync.RWMutex`
6. ✅ Removed unnecessary types: `FdDependency`, `ChainResult`

**Result**: Pipeline management dramatically simplified. Parent processes now follow the correct approach: create pipe, start child, immediately close handed-off FDs. No complex chain tracking needed.

### Phase 2: VFS Pre-population Enhancement ✅ COMPLETED
1. ✅ Enhanced Engine initialization to register CLI files in VFS at startup
2. ✅ Updated `app.go` to pass input/output files to VFS via RegisterRealFile()
3. ✅ Simplified NewEngine to use pure VFS (removed filesystem fallbacks)
4. ✅ Added verbose logging for VFS file registration
5. ✅ Eliminated VFS/RealFS switching logic in tool engine

**Key Implementation**:
- CLI -i files automatically registered with `RegisterRealFile(filename, O_RDONLY, 0644)`
- CLI -o files automatically registered with `RegisterRealFile(filename, O_WRONLY|O_CREATE|O_TRUNC, 0644)`
- Tool engine now purely VFS-based: `virtualFS.OpenFile()` only, no filesystem fallback
- Error handling: Clear messages when VFS unavailable or file not pre-registered

**Result**: LLM can now open CLI files by name directly via `open("input.txt", "r")` through pure VFS interface. No complex path detection or backend switching needed.### Phase 3: Testing & Validation
1. Test pure VFS file operations
2. Test simplified pipeline management
3. Verify no regressions in existing functionality

## Files to Modify

1. `internal/tools/engine.go` - Simplify executeSpawn FD management
2. `internal/app/app.go` - Enhance VFS initialization with CLI files  
3. `internal/app/vfs.go` - Ensure robust VFS file registration
4. Tests - Update for simplified behavior

## Expected Benefits

1. **Reduced Complexity**: No complex FD chain tracking
2. **Cleaner API**: Pure VFS operations throughout
3. **Better Performance**: Simpler resource management
4. **Easier Maintenance**: Less complex pipeline logic
