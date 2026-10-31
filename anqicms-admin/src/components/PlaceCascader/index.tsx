import {
  pluginPlaceGetCities,
  pluginPlaceGetCountries,
  pluginPlaceGetStates,
} from '@/services';
import { ProForm, ProFormItemProps } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import type { CascaderProps } from 'antd';
import { Cascader } from 'antd';
import React, { useCallback, useEffect, useRef, useState } from 'react';

export interface PlaceItem {
  id?: number;
  country_code: string;
  province_code?: string;
  city_code?: string;
  country: string;
  province?: string;
  city?: string;
  address?: string;
  latitude?: string;
  longitude?: string;
}

export interface PlaceCascaderProps extends ProFormItemProps {
  multiple?: boolean;
  placeholder?: string;
  disabled?: boolean;
  showSearch?: boolean;
  value?: PlaceItem | PlaceItem[];
  onChange?: (value: PlaceItem | PlaceItem[]) => void;
  cascaderProps?: Omit<
    CascaderProps<any>,
    | 'value'
    | 'onChange'
    | 'loadData'
    | 'options'
    | 'loading'
    | 'displayRender'
    | 'multiple'
    | 'changeOnSelect'
  > & {
    displayRender?: (
      labels: string[],
      selectedOptions?: CascaderOption[],
    ) => React.ReactNode;
    placeholder?: string;
    disabled?: boolean;
    showSearch?: boolean;
    multiple?: boolean;
  };
}

interface CascaderOption {
  value: string;
  label: string;
  isLeaf?: boolean;
  children?: CascaderOption[];
  latitude?: string;
  longitude?: string;
  level?: number;
}

const PlaceCascader: React.FC<PlaceCascaderProps> = ({
  label,
  name,
  multiple = false,
  placeholder,
  disabled = false,
  showSearch = false,
  rules,
  onChange,
  cascaderProps,
  ...restProps
}) => {
  const intl = useIntl();
  const resolvedPlaceholder =
    placeholder ??
    intl.formatMessage({ id: 'component.placeCascader.placeholder' });
  const resolvedRules =
    rules ??
    [
      {
        required: true,
        message: intl.formatMessage({
          id: 'component.placeCascader.required',
        }),
      },
    ];
  const [options, setOptions] = useState<CascaderOption[]>([]);
  const [loading, setLoading] = useState(false);
  const optionsRef = useRef<CascaderOption[]>([]);
  const multipleRef = useRef(multiple);

  useEffect(() => {
    multipleRef.current = multiple;
  }, [multiple]);

  useEffect(() => {
    optionsRef.current = options;
  }, [options]);

  const loadCountries = async () => {
    try {
      const res = await pluginPlaceGetCountries({});
      return res.data || [];
    } catch (e) {
      return [];
    }
  };

  const loadStates = async (countryCode: string) => {
    try {
      const res = await pluginPlaceGetStates({ country_code: countryCode });
      return res.data || [];
    } catch (e) {
      return [];
    }
  };

  const loadCities = async (countryCode: string, stateCode?: string) => {
    try {
      const res = await pluginPlaceGetCities({
        country_code: countryCode,
        state_code: stateCode,
      });
      return res.data || [];
    } catch (e) {
      return [];
    }
  };

  const pathToPlaceItem = useCallback((path: string[]): PlaceItem => {
    const item: PlaceItem = {
      country_code: '',
      country: '',
    };

    if (!path || path.length === 0) return item;

    const currentOptions = optionsRef.current;
    const findNode = (
      opts: CascaderOption[],
      val: string,
    ): CascaderOption | null => {
      for (const node of opts) {
        if (node.value === val) return node;
        if (node.children) {
          const found = findNode(node.children, val);
          if (found) return found;
        }
      }
      return null;
    };

    const countryNode = findNode(currentOptions, path[0]);
    if (countryNode) {
      item.country_code = countryNode.value;
      item.country = countryNode.label;
      item.latitude = countryNode.latitude;
      item.longitude = countryNode.longitude;
    }

    if (path.length > 1 && countryNode?.children) {
      const stateNode = findNode(countryNode.children, path[1]);
      if (stateNode) {
        item.province_code = stateNode.value;
        item.province = stateNode.label;
        item.latitude = stateNode.latitude;
        item.longitude = stateNode.longitude;
      }

      if (path.length > 2 && stateNode?.children) {
        const cityNode = findNode(stateNode.children, path[2]);
        if (cityNode) {
          item.city_code = cityNode.value;
          item.city = cityNode.label;
          item.latitude = cityNode.latitude;
          item.longitude = cityNode.longitude;
        }
      }
    }

    return item;
  }, []);

  const placeItemToPath = useCallback((item: PlaceItem): string[] => {
    const path: string[] = [];
    if (item.country_code) {
      path.push(item.country_code);
      if (item.province_code) {
        path.push(item.province_code);
        if (item.city_code) {
          path.push(item.city_code);
        }
      }
    }
    return path;
  }, []);

  const normalizeValue = useCallback(
    (val: PlaceItem | PlaceItem[] | undefined): string[][] | string[] => {
      if (!val) return multipleRef.current ? [] : [];

      if (multipleRef.current) {
        const items = Array.isArray(val) ? val : [val];
        return items.map((item) => placeItemToPath(item));
      }

      const item = Array.isArray(val) ? val[0] : val;
      return placeItemToPath(item);
    },
    [placeItemToPath],
  );

  const loadCountriesAndInit = async () => {
    const countryData = await loadCountries();
    const opts: CascaderOption[] = countryData.map((c: any) => ({
      label: c.name,
      value: c.iso2,
      latitude: c.latitude,
      longitude: c.longitude,
      isLeaf: false,
      level: 0,
    }));
    setOptions(opts);
  };

  const updateNodeChildren = (
    opts: CascaderOption[],
    parentValue: string,
    children: CascaderOption[],
  ): CascaderOption[] => {
    return opts.map((node) => {
      if (node.value === parentValue) {
        return { ...node, children };
      }
      if (node.children) {
        return {
          ...node,
          children: updateNodeChildren(node.children, parentValue, children),
        };
      }
      return node;
    });
  };

  const loadData: CascaderProps<any>['loadData'] = async (selectedOptions) => {
    const targetOption = selectedOptions[selectedOptions.length - 1];
    const level = targetOption.level ?? selectedOptions.length - 1;
    setLoading(true);

    let children: CascaderOption[] = [];

    if (level === 0) {
      const states = await loadStates(targetOption.value);
      children = states.map((s: any) => ({
        label: s.name,
        value: s.iso2,
        latitude: s.latitude,
        longitude: s.longitude,
        isLeaf: false,
        level: 1,
      }));
    } else if (level === 1) {
      const countryCode = selectedOptions[0]?.value;
      const stateCode = targetOption.value;
      const cities = await loadCities(countryCode, stateCode);
      children = cities.map((c: any) => ({
        label: c.name,
        value: c.name,
        latitude: c.latitude,
        longitude: c.longitude,
        isLeaf: true,
        level: 2,
      }));
    }

    setOptions((prev) =>
      updateNodeChildren(prev, targetOption.value, children),
    );
    setLoading(false);
  };

  const displayRender = (
    labels: string[],
    selectedOptions?: CascaderOption[],
  ) => {
    if (cascaderProps?.displayRender) {
      return cascaderProps.displayRender(labels, selectedOptions);
    }
    return labels.join('/');
  };

  useEffect(() => {
    loadCountriesAndInit();
  }, []);

  const handleCascaderChange = (cascaderValue: any) => {
    let newValue: PlaceItem | PlaceItem[];

    if (multipleRef.current) {
      const paths = cascaderValue as string[][];
      newValue = paths.map((path) => pathToPlaceItem(path));
    } else {
      const paths = cascaderValue as string[];
      newValue = pathToPlaceItem(paths);
    }

    onChange?.(newValue);
  };

  return (
    <ProForm.Item
      label={label}
      name={name}
      rules={resolvedRules}
      valuePropName="value"
      getValueFromEvent={(e: any) => {
        const cascaderValue = e;
        if (multipleRef.current) {
          const paths = cascaderValue as string[][];
          return paths.map((path) => pathToPlaceItem(path));
        } else {
          const paths = cascaderValue as string[];
          return pathToPlaceItem(paths);
        }
      }}
      getValueProps={(formValue: any) => {
        return {
          value: normalizeValue(formValue),
        };
      }}
      {...restProps}
    >
      {/* @ts-expect-error - multiple prop type narrowing issue */}
      <Cascader
        {...{ multiple }}
        options={options}
        loadData={loadData}
        loading={loading}
        changeOnSelect
        placeholder={resolvedPlaceholder}
        displayRender={displayRender}
        disabled={disabled}
        showSearch={showSearch}
        onChange={handleCascaderChange}
        {...cascaderProps}
      />
    </ProForm.Item>
  );
};

export default PlaceCascader;
