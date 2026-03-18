import type { Repository, Organization, Enterprise } from '$lib/api/generated/api.js';

type Entity = Repository | Organization | Enterprise;

/**
 * Determine the forge type of an entity (e.g. 'github', 'gitea').
 * Checks forge_type and endpoint.endpoint_type properties.
 */
export function getEntityForgeType(entity: Entity | null | undefined): string | null {
	if (!entity) return null;

	if ('forge_type' in entity && entity.forge_type) {
		return entity.forge_type as string;
	}
	if ('endpoint' in entity) {
		const endpoint = (entity as Record<string, unknown>).endpoint;
		if (endpoint && typeof endpoint === 'object' && 'endpoint_type' in endpoint) {
			return ((endpoint as Record<string, unknown>).endpoint_type as string) || null;
		}
	}
	// Default assumption for entities without explicit forge type
	return 'github';
}

/**
 * Check if an entity has agent mode enabled.
 */
export function getEntityAgentMode(entity: Entity | null | undefined): boolean {
	if (!entity) return false;

	if ('agent_mode' in entity) {
		return (entity.agent_mode as boolean) ?? false;
	}
	return false;
}

/**
 * Find an entity by ID in a list and return its forge type.
 */
export function getEntityForgeTypeById(
	entityId: string,
	entities: Entity[]
): string | null {
	if (!entityId || !entities) return null;
	const entity = entities.find((e) => e.id === entityId);
	return getEntityForgeType(entity);
}

/**
 * Find an entity by ID in a list and return its agent mode.
 */
export function getEntityAgentModeById(
	entityId: string,
	entities: Entity[]
): boolean {
	if (!entityId || !entities) return false;
	const entity = entities.find((e) => e.id === entityId);
	return getEntityAgentMode(entity);
}
