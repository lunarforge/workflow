import { writable } from 'svelte/store';

export type Density = 'comfortable' | 'compact';

export const density = writable<Density>('comfortable');
export const subsystemScope = writable<string>('');
