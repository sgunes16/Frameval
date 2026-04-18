import Editor from '@monaco-editor/react';

export function ArtifactEditor({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return <Editor height="320px" defaultLanguage="markdown" value={value} onChange={(next) => onChange(next ?? '')} />;
}
