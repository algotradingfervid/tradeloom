import { NavLink } from 'react-router-dom';
import { Icon } from '../components/Icon';
import { Tooltip } from '../components/Tooltip';
import { topBarCopy } from '../i18n/copy';

/** FR-SHELL-006. The Notifications bell joins in P3 (isEnabled('notificationsBell')). */
export function QuickLinks() {
  return (
    <>
      <Tooltip label={topBarCopy.ordersTip}>
        <NavLink to="/orders" className="toplink">
          <Icon name="orders" size={16} />
          <span>{topBarCopy.orders}</span>
        </NavLink>
      </Tooltip>
      <Tooltip label={topBarCopy.positionsTip}>
        <NavLink to="/positions" className="toplink">
          <Icon name="positions" size={16} />
          <span>{topBarCopy.positions}</span>
        </NavLink>
      </Tooltip>
    </>
  );
}
