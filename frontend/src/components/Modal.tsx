import type { ReactNode } from "react";
import { createPortal } from "react-dom";

import { IconX } from "./icons";
import { IconButton } from "./ui";

interface ModalProps {
  title: string;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  width?: number;
}

export function Modal({ title, onClose, children, footer, width = 480 }: ModalProps) {
  return createPortal(
    <div className="modal-overlay" onMouseDown={onClose}>
      <div
        className="modal-panel"
        style={{ width }}
        onMouseDown={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <div className="card-header">
          <h3>{title}</h3>
          <IconButton label="Close" onClick={onClose}>
            <IconX width={14} height={14} />
          </IconButton>
        </div>
        <div className="card-body modal-body">{children}</div>
        {footer && <div className="modal-footer">{footer}</div>}
      </div>
    </div>,
    document.body,
  );
}
