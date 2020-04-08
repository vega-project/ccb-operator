import React, { useState } from 'react';
import { Page, PageHeader, PageSidebar, PageSection, Brand } from '@patternfly/react-core';
import logo from '../../../App/static/images/vega-logo.png';
import Dashboard from '../Dashboard/Dashboard';

const AppLayout = () => {
  const [isNavOpen, setNavOpen] = useState(true);

  const logoProps = {
    href: '../../assets/vega-logo.png',
    onClick: () => console.log('clicked logo'),
    target: '_blank'
  };

  const Header = (
    <PageHeader
      logo={<Brand src={logo} alt="Vega Logo Black" />}
      logoProps={logoProps}
      showNavToggle
      isNavOpen={isNavOpen}
      onNavToggle={() => setNavOpen(!isNavOpen)}
    />
  );
  const Sidebar = <PageSidebar nav="Dashboard" isNavOpen={isNavOpen} theme="dark" />;

  return (
    <Page header={Header} sidebar={Sidebar}>
      <PageSection>
        <Dashboard />
      </PageSection>
    </Page>
  );
};

export default AppLayout;
