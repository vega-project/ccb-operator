import React, { useState } from 'react';
import {
    Avatar,
    Brand,
    Dropdown,
    DropdownGroup,
    DropdownToggle,
    DropdownItem,
    PageHeader,
    PageHeaderTools,
    PageHeaderToolsGroup,
    PageHeaderToolsItem
  } from '@patternfly/react-core';import Navigation from './Navigation'
import logo from '../../../public/vega-logo.png';

const userDropdownItems = [
    <DropdownGroup key="group 2">
      <DropdownItem key="group 2 profile">My profile</DropdownItem>
      <DropdownItem key="group 2 logout">Logout</DropdownItem>
    </DropdownGroup>
  ];

const HeaderTools = () => {
    const [isDropdownOpen, setIsDropdownOpen] = useState(false);

    return (
        <PageHeaderTools>
            <PageHeaderToolsGroup>
                <PageHeaderToolsItem>
                    <Dropdown
                        isPlain
                        position="right"
                        isOpen={isDropdownOpen}
                        toggle={<DropdownToggle onToggle={() => setIsDropdownOpen(!isDropdownOpen)}>John Doe</DropdownToggle>}
                        dropdownItems={userDropdownItems}
                    />  
                </PageHeaderToolsItem>
            </PageHeaderToolsGroup>
        </PageHeaderTools>
  );
}

const Header = (
    <PageHeader logo={<Brand src={logo} alt="Vega Logo" />} headerTools={<HeaderTools />} topNav={<Navigation />} />
);

export default Header;