import { defineStore } from 'pinia';

export type TaskType = 'search' | 'download' | 'save';
export type TaskStatus = 'queued' | 'active' | 'done' | 'timed-out' | 'failed';

export interface TaskEvent {
  time: number;      // Date.now()
  message: string;
  level: 'info' | 'warn' | 'error';
}

export interface Task {
  id: string;
  type: TaskType;
  status: TaskStatus;
  label: string;
  createdAt: number;
  updatedAt: number;
  events: TaskEvent[];
  meta: Record<string, unknown>;
  activeAt?: number;    // Unix ms when task became the active (IRC-searching) entry
  // convenience fields:
  query?: string;       // search
  bookTitle?: string;   // download
  server?: string;      // download
  resultCount?: number; // search
  errorCount?: number;  // search
  phase?: string;       // download phase
}

export const useTaskStore = defineStore('tasks', {
  state: () => ({
    tasks: [] as Task[],
  }),
  getters: {
    activeTasks: (state) => state.tasks.filter(t => t.status === 'active' || t.status === 'queued'),
    activeCount: (state) => state.tasks.filter(t => t.status === 'active' || t.status === 'queued').length,
    recentTasks: (state) => [...state.tasks].sort((a, b) => b.updatedAt - a.updatedAt).slice(0, 50),
  },
  actions: {
    createTask(type: TaskType, label: string, extra?: Partial<Task>): Task {
      const task: Task = {
        id: crypto.randomUUID(),
        type,
        status: type === 'download' ? 'queued' : 'active',
        label,
        createdAt: Date.now(),
        updatedAt: Date.now(),
        events: [{ time: Date.now(), message: type === 'download' ? 'Queued' : 'Started', level: 'info' }],
        meta: {},
        ...extra,
      };
      this.tasks.unshift(task);
      if (this.tasks.length > 200) this.tasks.splice(200);
      return task;
    },
    updateTask(id: string, updates: Partial<Task>, eventMessage?: string) {
      const task = this.tasks.find(t => t.id === id);
      if (!task) return;
      Object.assign(task, updates, { updatedAt: Date.now() });
      if (eventMessage) {
        task.events.push({ time: Date.now(), message: eventMessage, level: 'info' });
      }
    },
    addEvent(id: string, message: string, level: TaskEvent['level'] = 'info') {
      const task = this.tasks.find(t => t.id === id);
      if (!task) return;
      task.events.push({ time: Date.now(), message, level });
      task.updatedAt = Date.now();
    },
    findActive(type: TaskType): Task | undefined {
      return this.tasks.find(t => t.type === type && (t.status === 'active' || t.status === 'queued'));
    },
    findDownload(bookTitle?: string): Task | undefined {
      return this.tasks.find(t => t.type === 'download' && (t.status === 'active' || t.status === 'queued') && (!bookTitle || t.bookTitle === bookTitle));
    },
    clearCompleted() {
      this.tasks = this.tasks.filter(t => t.status === 'active' || t.status === 'queued');
    },
  },
});
