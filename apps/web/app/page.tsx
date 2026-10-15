export default function Home() {
  return (
    <main className="mx-auto max-w-xl px-6 py-20">
      <h1 className="text-3xl font-semibold tracking-tight">LaunchBase</h1>
      <p className="mt-4 text-slate-300">
        Dashboard is not built yet. This page just means the web app started.
      </p>
      <p className="mt-6 text-sm text-slate-400">
        API health is{" "}
        <a className="text-sky-300 underline" href="http://localhost:8080/health">
          localhost:8080/health
        </a>
        .
      </p>
    </main>
  );
}
