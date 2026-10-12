import type { ComponentPropsWithRef } from 'react';
import { Icon, type IconName } from './Icon';
import { Tooltip } from './Tooltip';
import './components.css';

export interface IconButtonProps extends Omit<ComponentPropsWithRef<'button'>, 'children' | 'aria-label'> {
  label: string;
  icon: IconName;
  tooltip?: string;
  size?: number;
}

export function IconButton({ label, icon, tooltip, size = 18, className, type = 'button', ...rest }: IconButtonProps) {
  return (
    <Tooltip label={tooltip ?? label}>
      <button type={type} aria-label={label} className={['icon-btn', className].filter(Boolean).join(' ')} {...rest}>
        <Icon name={icon} size={size} />
      </button>
    </Tooltip>
  );
}
