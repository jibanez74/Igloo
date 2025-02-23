import { useEffect, useRef, useState, forwardRef } from "react";

type DropdownProps = {
  trigger: React.ReactNode;
  items: {
    label: string;
    icon?: React.ReactNode;
    onClick: () => void;
  }[];
  align?: "left" | "right";
  label: string;
};

const Dropdown = forwardRef<HTMLDivElement, DropdownProps>(
  ({ trigger, items, align = "right", label }, ref) => {
    const [isOpen, setIsOpen] = useState(false);
    const [activeIndex, setActiveIndex] = useState(-1);
    const menuItemsRef = useRef<(HTMLButtonElement | null)[]>([]);
    const internalRef = useRef<HTMLDivElement>(null);

    // Combine refs properly
    const dropdownRef = (ref as React.RefObject<HTMLDivElement>) || internalRef;

    useEffect(() => {
      const handleClickOutside = (event: MouseEvent) => {
        // Check if dropdownRef exists and has current property
        const element = (dropdownRef as React.RefObject<HTMLDivElement>)
          .current;
        if (element && !element.contains(event.target as Node)) {
          setIsOpen(false);
          setActiveIndex(-1);
        }
      };

      document.addEventListener("mousedown", handleClickOutside);
      return () =>
        document.removeEventListener("mousedown", handleClickOutside);
    }, [dropdownRef]); // Add dropdownRef to dependency array

    // Focus management
    useEffect(() => {
      if (isOpen && activeIndex >= 0) {
        menuItemsRef.current[activeIndex]?.focus();
      }
    }, [isOpen, activeIndex]);

    const handleKeyDown = (e: React.KeyboardEvent) => {
      if (!isOpen) {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          setIsOpen(true);
          setActiveIndex(0);
        }
        return;
      }

      switch (e.key) {
        case "ArrowDown":
          e.preventDefault();
          setActiveIndex(prev => (prev < items.length - 1 ? prev + 1 : 0));
          break;
        case "ArrowUp":
          e.preventDefault();
          setActiveIndex(prev => (prev > 0 ? prev - 1 : items.length - 1));
          break;
        case "Home":
          e.preventDefault();
          setActiveIndex(0);
          break;
        case "End":
          e.preventDefault();
          setActiveIndex(items.length - 1);
          break;
        case "Escape":
          e.preventDefault();
          setIsOpen(false);
          setActiveIndex(-1);
          break;
        case "Tab":
          setIsOpen(false);
          setActiveIndex(-1);
          break;
      }
    };

    return (
      <div className='relative' ref={ref} onKeyDown={handleKeyDown}>
        <div
          role='button'
          aria-haspopup='true'
          aria-expanded={isOpen}
          aria-label={label}
          onClick={() => {
            setIsOpen(!isOpen);
            if (!isOpen) setActiveIndex(0);
          }}
          tabIndex={0}
        >
          {trigger}
        </div>

        {isOpen && (
          <div
            className={`absolute z-50 ${
              align === "right" ? "right-0" : "left-0"
            } mt-2 w-48 rounded-lg bg-slate-800 shadow-lg ring-1 ring-black ring-opacity-5`}
            role='menu'
            aria-orientation='vertical'
            aria-label={`${label} options`}
          >
            <div className='py-1'>
              {items.map((item, index) => (
                <button
                  key={index}
                  ref={(element: HTMLButtonElement | null) => {
                    menuItemsRef.current[index] = element;
                  }}
                  onClick={() => {
                    item.onClick();
                    setIsOpen(false);
                    setActiveIndex(-1);
                  }}
                  className={`flex w-full items-center px-4 py-2 text-sm text-sky-200 
                           hover:bg-slate-700 hover:text-sky-100 transition-colors
                           ${activeIndex === index ? "bg-slate-700 text-sky-100" : ""}`}
                  role='menuitem'
                  aria-label={item.label}
                  tabIndex={isOpen ? 0 : -1}
                >
                  {item.icon && (
                    <span className='mr-3' aria-hidden='true'>
                      {item.icon}
                    </span>
                  )}
                  {item.label}
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    );
  }
);

Dropdown.displayName = "Dropdown";

export default Dropdown;
