import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import type { Variant } from '../../lib/types';

export function StepVariants({ variants, onChange }: { variants: Variant[]; onChange: (variants: Variant[]) => void }) {
  return (
    <div className="space-y-3">
      {variants.map((variant, index) => (
        <Input key={variant.id || index} value={variant.name} onChange={(event) => onChange(variants.map((item, itemIndex) => itemIndex === index ? { ...item, name: event.target.value } : item))} placeholder={`Variant ${index + 1}`} />
      ))}
      <Button type="button" onClick={() => onChange([...variants, { id: '', experiment_id: '', name: '', description: '', is_control: variants.length === 0, ordering: variants.length }])}>Add variant</Button>
    </div>
  );
}
