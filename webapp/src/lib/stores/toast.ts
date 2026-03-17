import { writable } from 'svelte/store';

export interface Toast {
    id: string;
    type: 'success' | 'error' | 'info' | 'warning';
    title: string;
    message: string;
    duration?: number; // milliseconds, 0 for manual dismiss
}

function createToastStore() {
    const { subscribe, set, update } = writable<Toast[]>([]);
    const timers = new Map<string, ReturnType<typeof setTimeout>>();

    const store = {
        subscribe,
        add: (toast: Omit<Toast, 'id'>) => {
            const id = Math.random().toString(36).substring(2, 11);
            const newToast: Toast = {
                ...toast,
                id,
                duration: toast.duration ?? 5000
            };

            update(toasts => [...toasts, newToast]);

            // Auto-remove after duration
            if (newToast.duration && newToast.duration > 0) {
                const timer = setTimeout(() => {
                    timers.delete(id);
                    update(toasts => toasts.filter(t => t.id !== id));
                }, newToast.duration);
                timers.set(id, timer);
            }

            return id;
        },
        remove: (id: string) => {
            // Clear auto-remove timer if toast is dismissed early
            const timer = timers.get(id);
            if (timer) {
                clearTimeout(timer);
                timers.delete(id);
            }
            update(toasts => toasts.filter(t => t.id !== id));
        },
        clear: () => {
            // Clear all pending timers
            timers.forEach(timer => clearTimeout(timer));
            timers.clear();
            set([]);
        },
        success: (title: string, message: string = '', duration?: number) => {
            return store.add({ type: 'success', title, message, duration });
        },
        error: (title: string, message: string = '', duration?: number) => {
            return store.add({ type: 'error', title, message, duration });
        },
        info: (title: string, message: string = '', duration?: number) => {
            return store.add({ type: 'info', title, message, duration });
        },
        warning: (title: string, message: string = '', duration?: number) => {
            return store.add({ type: 'warning', title, message, duration });
        }
    };

    return store;
}

export const toastStore = createToastStore();
