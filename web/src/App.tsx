function App() {
  return (
    <div className="min-h-screen bg-gray-950 text-gray-100 flex flex-col items-center justify-center">
      <header className="text-center">
        <h1 className="text-5xl font-bold tracking-tight mb-4">
          <span className="text-purple-400">Shadow</span>
        </h1>
        <p className="text-lg text-gray-400 max-w-md mx-auto">
          Your AI agent memory layer — correct once, remember everywhere.
        </p>
      </header>

      <main className="mt-12 grid grid-cols-1 md:grid-cols-3 gap-6 max-w-4xl w-full px-6">
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <div className="text-3xl mb-3">🪶</div>
          <h2 className="text-lg font-semibold mb-2">Auto Capture</h2>
          <p className="text-sm text-gray-400">
            Silently records every correction you make to your coding agents.
          </p>
        </div>
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <div className="text-3xl mb-3">🔮</div>
          <h2 className="text-lg font-semibold mb-2">Rule Distillation</h2>
          <p className="text-sm text-gray-400">
            Turns messy corrections into clean, structured rules automatically.
          </p>
        </div>
        <div className="bg-gray-900 rounded-xl p-6 border border-gray-800">
          <div className="text-3xl mb-3">🔌</div>
          <h2 className="text-lg font-semibold mb-2">Universal Adapter</h2>
          <p className="text-sm text-gray-400">
            Rules work in Claude Code, Cursor, Codex, and any other agent.
          </p>
        </div>
      </main>

      <footer className="mt-16 text-sm text-gray-600">
        Shadow v0.1.0 — Local-first, your data stays yours.
      </footer>
    </div>
  )
}

export default App
