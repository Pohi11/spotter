import ReactMarkdown from "react-markdown";

// Markdown from the repository's own files (specs, proposals, decision records). Links to
// other repository files open on GitHub; headings are scaled down to fit inside a card.

const REPO = "https://github.com/Pohi11/spotter/blob/main/";

function href(raw: string | undefined, base: string): string | undefined {
  if (!raw || /^[a-z]+:/i.test(raw) || raw.startsWith("#")) return raw;
  const parts = base.split("/").slice(0, -1);
  for (const seg of raw.split("/")) {
    if (seg === "..") parts.pop();
    else if (seg !== ".") parts.push(seg);
  }
  return REPO + parts.join("/");
}

export function Md({ children, base = "" }: { children: string; base?: string }) {
  return (
    <div className="space-y-3 leading-relaxed text-muted [&_code]:rounded [&_code]:bg-wash [&_code]:px-1 [&_code]:text-[0.85em] [&_code]:text-ink [&_li]:ml-5 [&_ol]:list-decimal [&_strong]:text-ink [&_ul]:list-disc">
      <ReactMarkdown
        components={{
          h1: ({ children }) => <h3 className="text-xl font-semibold text-ink">{children}</h3>,
          h2: ({ children }) => <h4 className="mt-5 text-lg font-semibold text-ink">{children}</h4>,
          h3: ({ children }) => <h5 className="mt-4 font-semibold text-ink">{children}</h5>,
          a: ({ href: h, children }) => (
            <a href={href(h, base)} className="text-code underline decoration-code/30 underline-offset-2" target="_blank" rel="noopener noreferrer">
              {children}
            </a>
          ),
          table: ({ children }) => (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm [&_td]:border-t [&_td]:border-rule [&_td]:py-1.5 [&_td]:pr-3 [&_th]:py-1.5 [&_th]:pr-3 [&_th]:font-medium [&_th]:text-ink">
                {children}
              </table>
            </div>
          ),
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
