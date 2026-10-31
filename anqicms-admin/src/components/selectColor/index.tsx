import { cyan, generate, green, presetPalettes, red } from '@ant-design/colors';
import type { ColorPickerProps } from 'antd';
import { Col, ColorPicker, Divider, Row, theme } from 'antd';
import { Color } from 'antd/es/color-picker';

export type SelectColorProps = {
  defaultValue: string;
  showText: boolean | ((color: Color) => React.ReactNode);
  onChangeComplete: (value: Color) => void;
};

type Presets = Required<ColorPickerProps>['presets'][number];

function genPresets(presets = presetPalettes) {
  return Object.entries(presets).map<Presets>(([label, colors]) => ({
    label,
    colors,
    key: label,
  }));
}

const SelectColor: React.FC<SelectColorProps> = (props) => {
  const { token } = theme.useToken();

  const presets = genPresets({
    primary: generate(token.colorPrimary),
    red,
    green,
    cyan,
  });

  const customPanelRender: ColorPickerProps['panelRender'] = (
    _,
    { components: { Picker, Presets } },
  ) => (
    <Row justify="space-between" wrap={false}>
      <Col span={12}>
        <Presets />
      </Col>
      <Divider type="vertical" style={{ height: 'auto' }} />
      <Col flex="auto">
        <Picker />
      </Col>
    </Row>
  );

  return (
    <ColorPicker
      defaultValue={props.defaultValue}
      showText={props.showText}
      onChangeComplete={(e) => props.onChangeComplete(e)}
      styles={{ popupOverlayInner: { width: 500 } }}
      presets={presets}
      panelRender={customPanelRender}
    />
  );
};

export default SelectColor;
