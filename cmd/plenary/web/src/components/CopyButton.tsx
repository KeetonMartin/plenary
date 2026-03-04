import { useState } from "react";
import { copyToClipboard } from "../utils";

export function CopyButton({ text, label }: { text: string; label?: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async (e: React.MouseEvent) => {
    e.stopPropagation();
    const ok = await copyToClipboard(text);
    if (ok) {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    }
  };

  return (
    <button
      onClick={handleCopy}
      className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors font-mono cursor-pointer"
      title={`Copy ${label || text}`}
    >
      <span className="truncate max-w-40">{text}</span>
      <span className="shrink-0 w-4 text-center">
        {copied ? "\u2713" : "\u2398"}
      </span>
    </button>
  );
}
