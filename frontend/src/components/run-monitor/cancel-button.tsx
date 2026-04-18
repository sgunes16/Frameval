import { Button } from '../../components/ui/button';

export function CancelButton({ onClick }: { onClick: () => void }) {
  return (
    <Button variant="danger" size="sm" onClick={onClick}>
      Cancel
    </Button>
  );
}
