import { writable } from 'svelte/store';

export interface SubsystemInfo {
	id: string;
	name: string;
}

export const subsystems = writable<SubsystemInfo[]>([]);
