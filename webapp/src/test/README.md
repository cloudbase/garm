# GARM Webapp Unit Tests

This directory contains unit tests for the GARM webapp, focusing on testing individual components and utility functions.

## Test Structure

### Setup Files
- `setup.ts` - Global test setup and mocks for SvelteKit modules
- `mocks.ts` - Mock factories for API clients, stores, and external dependencies
- `factories.ts` - Factory functions for creating test data objects

### Test Files
- `src/routes/repositories/page.test.ts` - Comprehensive tests for the repositories list page

## Running Tests

```bash
# Run all tests once
npm run test:run

# Run tests in watch mode
npm run test

# Run tests with UI (if vitest UI is installed)
npm run test:ui
```

## Test Coverage

The repositories page tests cover:

### Page Loading
- ✅ Page title rendering
- ✅ Loading state management
- ✅ Error handling during data fetching
- ✅ Cache manager integration

### Repository List Rendering
- ✅ Repository data display
- ✅ GitHub forge icon rendering
- ✅ Gitea forge icon rendering
- ✅ Status badge generation
- ✅ Column configuration
- ✅ Mobile card configuration

### Search and Filtering
- ✅ Repository filtering by name
- ✅ Repository filtering by owner
- ✅ Search term handling
- ✅ Empty search results

### Pagination
- ✅ Page navigation
- ✅ Items per page changes
- ✅ Total pages calculation
- ✅ Pagination controls

### Action Buttons and Modals
- ✅ Edit repository action
- ✅ Delete repository action
- ✅ Create repository modal
- ✅ Modal state management

### Repository Operations
- ✅ Repository creation
- ✅ Repository creation with webhook installation
- ✅ Repository updates
- ✅ Repository deletion
- ✅ Webhook installation

### Error Handling
- ✅ API error handling
- ✅ Creation error handling
- ✅ Webhook installation error handling
- ✅ Cache error handling

### Toast Notifications
- ✅ Success notifications
- ✅ Error notifications
- ✅ Operation feedback

### Cache Management
- ✅ Cache retry functionality
- ✅ Cache state management

## Testing Strategy

The tests follow these principles:

1. **Unit Testing Focus**: Tests focus on isolated functionality rather than full component integration
2. **Mock External Dependencies**: All API calls, stores, and external utilities are mocked
3. **Test Behavior, Not Implementation**: Tests verify expected behavior and user interactions
4. **Comprehensive Coverage**: Tests cover happy paths, error scenarios, and edge cases
5. **Readable Test Names**: Test descriptions clearly explain what functionality is being tested

## Mock Strategy

- **API Client**: Mocked to simulate successful and failed operations
- **Stores**: Mocked to provide predictable state management
- **Utilities**: Mocked to test business logic independently
- **Components**: Heavy components are mocked to focus on page logic

This approach ensures fast, reliable tests that validate the repositories page functionality without depending on external services or complex component rendering.