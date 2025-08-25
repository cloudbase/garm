# Pool Creation Anti-Duplication Design

## Problem Fixed

The CreatePoolModal was causing duplicate pool creation because both the modal component AND parent components were making API calls to create pools. This resulted in two identical pools being created.

## Root Cause

1. **CreatePoolModal.svelte** always called the API (e.g., `garmApi.createRepositoryPool()`) in its `handleSubmit` function
2. **Parent components** (repository/organization/enterprise detail pages) also called the same API when handling the modal's submit event
3. Both calls succeeded, creating duplicate pools

## Solution Architecture

The fix implements **conditional API calling** based on usage context:

### Entity Detail Pages (repositories/[id], organizations/[id], enterprises/[id])
- **Modal Role**: Validate form, dispatch submit event with parameters
- **Parent Role**: Handle API call, show success/error messages, manage modal state
- **API Call Made By**: Parent component only

```typescript
// Modal logic
if (initialEntityType && initialEntityId) {
    // Entity pages: parent handles the API call
    dispatch('submit', params);
} else {
    // Global page: modal handles the API call
    await garmApi.createRepositoryPool(selectedEntityId, params);
    dispatch('submit', params);
}
```

### Global Pools Page (/pools)
- **Modal Role**: Collect entity selection, validate form, make API call, dispatch submit event
- **Parent Role**: Show success message, manage modal state only
- **API Call Made By**: Modal component only

## Implementation Details

### CreatePoolModal.svelte Changes
- Added conditional logic in `handleSubmit()` method
- Checks for `initialEntityType` and `initialEntityId` props
- Only makes API calls when these props are NOT provided (global page scenario)

### Parent Component Changes
- Repository detail page: Error handling improved
- Organization detail page: Error handling improved  
- Enterprise detail page: Error handling improved
- Global pools page: Receives submit event correctly

## Testing Strategy

### Unit Tests
- `CreatePoolModal.simple.test.ts`: Tests modal rendering and basic functionality
- `CreatePoolModal.test.ts`: Comprehensive API call prevention tests (needs Svelte 5 updates)

### Integration Tests
- `pool-creation-anti-duplication.test.ts`: Regression prevention and architecture verification

### Key Test Cases
1. **Entity page usage**: Verify modal does NOT call API
2. **Global page usage**: Verify modal DOES call API
3. **Single API call**: Ensure exactly one API call per pool creation
4. **Error handling**: Proper error handling in both scenarios

## Preventing Future Regressions

### For Developers
1. **Always check context** when adding new entity types
2. **Follow the pattern**: Use `initialEntityType` to determine API call responsibility
3. **Run tests** before committing modal or parent component changes

### Code Review Checklist
- [ ] Does the modal make conditional API calls?
- [ ] Do parent components handle their responsibilities correctly?
- [ ] Are there tests covering the new functionality?
- [ ] Is there exactly one source of API calls per scenario?

## Responsibility Matrix

| Scenario | Modal Responsibilities | Parent Responsibilities |
|----------|----------------------|------------------------|
| Entity Detail Page | • Validate form<br>• Dispatch submit event | • Make API call<br>• Handle success/error<br>• Manage modal state |
| Global Pools Page | • Validate form<br>• Make API call<br>• Dispatch submit event | • Handle success message<br>• Manage modal state |

## File Changes Summary

### Modified Files
- `src/lib/components/CreatePoolModal.svelte` - Added conditional API calling
- `src/routes/repositories/[id]/+page.svelte` - Fixed error handling
- `src/routes/organizations/[id]/+page.svelte` - Fixed error handling
- `src/routes/enterprises/[id]/+page.svelte` - Fixed error handling
- `src/routes/pools/+page.svelte` - Updated event handler

### New Test Files
- `src/lib/components/CreatePoolModal.simple.test.ts`
- `src/routes/repositories/[id]/pool-creation.test.ts`
- `src/routes/pools/pool-creation.test.ts`
- `src/integration/pool-creation-anti-duplication.test.ts`

## Error Scenarios Handled

1. **API failures from entity pages**: Parent shows toast, keeps modal open
2. **API failures from global page**: Modal handles error display
3. **Network errors**: Graceful degradation in both scenarios
4. **Validation errors**: Handled before API calls are made

## Performance Impact

- **Positive**: Reduces API calls by 50% (no duplicate calls)
- **Neutral**: No additional network requests or computational overhead
- **Improved**: Better user experience with consistent error handling