export interface DashboardPollingTask {
  interval: number;
  run: () => Promise<void> | void;
}

export interface DashboardPollingScheduler {
  setInterval: (handler: () => void, interval: number) => unknown;
  clearInterval: (handle: unknown) => void;
}

const defaultScheduler: DashboardPollingScheduler = {
  setInterval: (handler, interval) => globalThis.setInterval(handler, interval),
  clearInterval: (handle) =>
    globalThis.clearInterval(handle as ReturnType<typeof setInterval>),
};

export function createDashboardPolling(
  tasks: DashboardPollingTask[],
  scheduler: DashboardPollingScheduler = defaultScheduler
) {
  const handles = new Map<DashboardPollingTask, unknown>();
  const inFlight = new Set<DashboardPollingTask>();
  let running = false;

  const execute = async (task: DashboardPollingTask) => {
    if (inFlight.has(task)) return;
    inFlight.add(task);
    try {
      await task.run();
    } finally {
      inFlight.delete(task);
    }
  };

  const start = () => {
    if (running) return;
    running = true;
    for (const task of tasks) {
      void execute(task);
      handles.set(
        task,
        scheduler.setInterval(() => void execute(task), task.interval)
      );
    }
  };

  const pause = () => {
    if (!running) return;
    running = false;
    for (const handle of handles.values()) scheduler.clearInterval(handle);
    handles.clear();
  };

  return {
    start,
    resume: start,
    pause,
    stop: pause,
    isRunning: () => running,
  };
}
