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

class Metalicity extends React.Component {
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
            <Radio isLabelWrapped label="+0.5dex" name="+0.5" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="0dex" name="0" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="-0.37dex" name="-0.37" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="-0.5dex" name="-0.5" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="-0.94dex" name="-0.94" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="-1dex" name="-1" />
        </DropdownItem>,
        <DropdownItem key="action" component="button">
            <Radio isLabelWrapped label="-1.5dex" name="-2" />
        </DropdownItem>
    ];
    return (
      <Dropdown
        onSelect={this.onSelect}
        toggle={
          <DropdownToggle id="toggle-id" onToggle={this.onToggle} toggleIndicator={CaretDownIcon}>
            [M/H]
          </DropdownToggle>
        }
        isOpen={isOpen}
        dropdownItems={dropdownItems}
      />
    );
  }
}

export default Metalicity;