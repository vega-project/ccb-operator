import React from 'react';
import {
    Dropdown,
    DropdownToggle,
    DropdownItem
} from '@patternfly/react-core';
import { CaretDownIcon } from '@patternfly/react-icons';
import { Radio } from '@patternfly/react-core';

class Vmicro extends React.Component {
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
                <Radio isLabelWrapped label="1 km/s" name="1" />
            </DropdownItem>,
            <DropdownItem key="action" component="button">
                <Radio isLabelWrapped label="2 km/s" name="2" />
            </DropdownItem>,
            <DropdownItem key="action" component="button">
                <Radio isLabelWrapped label="3 km/s" name="3" />
            </DropdownItem>,
            <DropdownItem key="action" component="button">
                <Radio isLabelWrapped label="4 km/s" name="4" />
            </DropdownItem>
        ];
        return (
            <Dropdown
                onSelect={this.onSelect}
                toggle={
                    <DropdownToggle id="toggle-id" onToggle={this.onToggle} toggleIndicator={CaretDownIcon}>
            Vmicro
                    </DropdownToggle>
                }
                isOpen={isOpen}
                dropdownItems={dropdownItems}
            />
        );
    }
}

export default Vmicro;
