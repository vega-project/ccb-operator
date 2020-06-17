import React from 'react';
import {
  Dropdown,
  DropdownToggle,
  DropdownItem,
  DropdownSeparator,
  DropdownPosition,
  DropdownDirection,
  KebabToggle
} from '@patternfly/react-core';
import { ThIcon, CaretDownIcon } from '@patternfly/react-icons';
import { Radio } from '@patternfly/react-core';

class Inclination extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      isOpen: false
    };
    this.onToggle = isOpen => {
      this.setState({
        isOpen
      });
    };
    this.onSelect = event => {
      this.setState({
        isOpen: !this.state.isOpen
      });
      this.onFocus();
    };
    this.onFocus = () => {
      const element = document.getElementById('toggle-id');
      element.focus();
    };
  }

  render() {
    const { isOpen } = this.state;
    const dropdownItems = [
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="5 deg" name="1" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="10 deg" name="2" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="20 deg" name="3" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="30 deg" name="4" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="40 deg" name="4" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="50 deg" name="4" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="60 deg" name="4" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="70 deg" name="4" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="80 deg" name="4" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="90 deg" name="4" />
        </DropdownItem>,
    ];
    return (
      <Dropdown
        onSelect={this.onSelect}
        toggle={
          <DropdownToggle id="toggle-id" onToggle={this.onToggle} toggleIndicator={CaretDownIcon}>
            Inclination
          </DropdownToggle>
        }
        isOpen={isOpen}
        dropdownItems={dropdownItems}
      />
    );
  }
}

export default Inclination;