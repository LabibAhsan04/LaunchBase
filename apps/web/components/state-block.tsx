type Props = {
  title: string;
  body: string;
  actionLabel?: string;
  onAction?: () => void;
};

export function StateBlock({ title, body, actionLabel, onAction }: Props) {
  return (
    <div className="rounded-xl border border-[var(--line)] bg-[var(--panel)] px-6 py-10 text-center">
      <h2 className="text-lg font-medium text-[var(--text)]">{title}</h2>
      <p className="mx-auto mt-2 max-w-md text-sm text-[var(--muted)]">{body}</p>
      {actionLabel && onAction ? (
        <button type="button" className="btn-secondary mt-6" onClick={onAction}>
          {actionLabel}
        </button>
      ) : null}
    </div>
  );
}

export function LoadingBlock({ label = "Loading…" }: { label?: string }) {
  return (
    <div className="rounded-xl border border-[var(--line)] bg-[var(--panel)] px-6 py-10 text-center text-sm text-[var(--muted)]">
      {label}
    </div>
  );
}
