import { Button } from '../../components/ui/button';

export function ExportButton({ href }: { href: string }) {
  return <a href={href}><Button>Export JSON</Button></a>;
}
