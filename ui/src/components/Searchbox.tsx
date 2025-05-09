import { MagnifyingGlassIcon } from '@heroicons/react/24/outline';
import { useEffect, useState } from 'react';

type SearchboxProps = {
  value: string;
  onChange: (value: string) => void;
  debounce?: number;
  className?: string;
};

export default function Searchbox(props: SearchboxProps) {
  const {
    value: initialValue,
    onChange,
    debounce = 500,
    className = ''
  } = props;
  const [value, setValue] = useState<string>(initialValue);

  useEffect(() => {
    setValue(initialValue);
  }, [initialValue]);

  useEffect(() => {
    const timeout = setTimeout(() => {
      onChange(value);
    }, debounce);

    return () => clearTimeout(timeout);
  }, [debounce, onChange, value]);

  return (
    <div className={`${className} flex flex-1 items-center justify-start`}>
      <div className="w-full max-w-60 lg:max-w-md">
        <label htmlFor="search" className="sr-only">
          Search
        </label>
        <div className="relative">
          <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
            <MagnifyingGlassIcon
              className="h-5 w-5 text-gray-400"
              aria-hidden="true"
            />
          </div>
          <input
            id="search"
            name="search"
            className="bg-background block w-full rounded-md border border-gray-300 py-2 pr-3 pl-10 leading-5 text-gray-900 placeholder-gray-500 shadow-xs focus:border-violet-400 focus:placeholder-gray-400 focus:ring-1 focus:ring-violet-400 focus:outline-hidden sm:text-sm"
            placeholder="Search"
            type="search"
            value={value}
            onChange={(e) => setValue(e.target.value)}
          />
        </div>
      </div>
    </div>
  );
}
