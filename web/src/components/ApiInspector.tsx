import React, { useState } from 'react'
import { ApiLogEntry } from '../types/api'
import { clearLogs } from '../services/apiClient'

interface ApiInspectorProps {
  logs: ApiLogEntry[]
  isOpen: boolean
  onClose: () => void
}

export const ApiInspector: React.FC<ApiInspectorProps> = ({ logs, isOpen, onClose }) => {
  const [selectedLogId, setSelectedLogId] = useState<string | null>(null)
  const [copiedKey, setCopiedKey] = useState<string | null>(null)

  if (!isOpen) return null

  const selectedLog = logs.find((l) => l.id === selectedLogId) || logs[0]

  const handleCopy = (text: string, key: string) => {
    navigator.clipboard.writeText(text)
    setCopiedKey(key)
    setTimeout(() => setCopiedKey(null), 1500)
  }

  return (
    <div className="fixed bottom-0 left-0 right-0 h-80 bg-[#141210] border-t border-stone-800 shadow-2xl z-50 flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 bg-[#191614] border-b border-stone-800 font-mono text-xs">
        <div className="flex items-center space-x-2">
          <span className="font-semibold text-stone-200 uppercase tracking-wider">
            API Inspector
          </span>
          <span className="text-stone-500">({logs.length} logged)</span>
        </div>
        <div className="flex items-center space-x-3">
          <button
            onClick={clearLogs}
            className="text-stone-400 hover:text-red-400 text-xs transition-colors cursor-pointer"
          >
            Clear
          </button>
          <button
            onClick={onClose}
            className="text-stone-400 hover:text-stone-200 text-xs transition-colors cursor-pointer"
          >
            Close [x]
          </button>
        </div>
      </div>

      {/* Body */}
      <div className="flex-1 flex overflow-hidden font-mono text-xs">
        {/* Left: Request List */}
        <div className="w-1/3 border-r border-stone-800 overflow-y-auto divide-y divide-stone-800/60 bg-[#12100e]">
          {logs.length === 0 ? (
            <div className="p-6 text-center text-stone-500 text-xs">
              No requests captured.
            </div>
          ) : (
            logs.map((log) => {
              const isSelected = selectedLog?.id === log.id
              return (
                <div
                  key={log.id}
                  onClick={() => setSelectedLogId(log.id)}
                  className={`p-2 cursor-pointer flex items-center justify-between transition-colors ${
                    isSelected
                      ? 'bg-[#26180f] text-[#fed7aa] border-l-2 border-[#854d0e]'
                      : 'hover:bg-stone-850 text-stone-400'
                  }`}
                >
                  <div className="truncate mr-2">
                    <span className="font-bold mr-1.5 text-stone-300">{log.method}</span>
                    <span className="text-stone-400">{log.url}</span>
                  </div>
                  <div className="flex items-center space-x-2 shrink-0">
                    <span className="text-[10px] text-stone-500">{log.durationMs}ms</span>
                    <span
                      className={`px-1 rounded text-[10px] ${
                        log.isError ? 'text-red-400' : 'text-emerald-400'
                      }`}
                    >
                      {log.status || 'ERR'}
                    </span>
                  </div>
                </div>
              )
            })
          )}
        </div>

        {/* Right: Request & Response Detail */}
        <div className="flex-1 flex flex-col bg-[#141210] overflow-y-auto p-3 space-y-3">
          {selectedLog ? (
            <>
              <div className="flex items-center justify-between p-2 rounded bg-[#191614] border border-stone-800 text-[11px]">
                <div className="space-x-2">
                  <span className="text-amber-300 font-bold">{selectedLog.method}</span>
                  <span className="text-stone-200">{selectedLog.url}</span>
                </div>
                <div className="space-x-3 text-stone-400">
                  <span>{selectedLog.timestamp}</span>
                  <span>{selectedLog.durationMs}ms</span>
                  <span className={selectedLog.isError ? 'text-red-400' : 'text-emerald-400'}>
                    HTTP {selectedLog.status}
                  </span>
                </div>
              </div>

              {selectedLog.requestBody && (
                <div>
                  <div className="flex items-center justify-between text-[10px] text-stone-400 mb-1 font-sans">
                    <span className="uppercase tracking-wider">Request Payload</span>
                    <button
                      onClick={() => handleCopy(JSON.stringify(selectedLog.requestBody, null, 2), 'req')}
                      className="text-stone-400 hover:text-stone-200"
                    >
                      {copiedKey === 'req' ? 'Copied' : 'Copy'}
                    </button>
                  </div>
                  <pre className="p-2.5 rounded bg-[#100e0c] border border-stone-800 text-stone-300 overflow-x-auto text-[11px]">
                    {JSON.stringify(selectedLog.requestBody, null, 2)}
                  </pre>
                </div>
              )}

              <div>
                <div className="flex items-center justify-between text-[10px] text-stone-400 mb-1 font-sans">
                  <span className="uppercase tracking-wider">Response Body</span>
                  <button
                    onClick={() => handleCopy(JSON.stringify(selectedLog.responseBody, null, 2), 'res')}
                    className="text-stone-400 hover:text-stone-200"
                  >
                    {copiedKey === 'res' ? 'Copied' : 'Copy'}
                  </button>
                </div>
                <pre className="p-2.5 rounded bg-[#100e0c] border border-stone-800 text-stone-300 overflow-x-auto text-[11px]">
                  {JSON.stringify(selectedLog.responseBody, null, 2)}
                </pre>
              </div>
            </>
          ) : (
            <div className="text-stone-500 text-center py-8">Select a request to inspect</div>
          )}
        </div>
      </div>
    </div>
  )
}
