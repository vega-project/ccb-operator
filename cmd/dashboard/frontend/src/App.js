import React from 'react';
import Header from './Components/Presentational/Header/Header';
import {
  Breadcrumb,
  BreadcrumbItem,
  Page,
  PageSection,
  PageSectionVariants,
  TextContent,
  Text,
} from '@patternfly/react-core';
import Routes from './Config/Routes'

const App = () => {

    const PageBreadcrumb = (
      <Breadcrumb>
        <BreadcrumbItem>Home</BreadcrumbItem>
        <BreadcrumbItem to="#" isActive>Grid</BreadcrumbItem>
      </Breadcrumb>
    );
    
    return (
      <Page header={Header}  breadcrumb={PageBreadcrumb}>
        <PageSection variant={PageSectionVariants.light}>
          <TextContent>
            <Text component="h1">Grid Model Selection</Text>
          </TextContent>
        </PageSection>
        <PageSection>
          <Routes />
        </PageSection>
      </Page>
    );
}

export default App;