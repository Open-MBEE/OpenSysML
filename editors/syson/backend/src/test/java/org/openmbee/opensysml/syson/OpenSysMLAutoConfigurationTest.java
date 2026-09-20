package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;

import org.eclipse.sirius.components.collaborative.api.IEditingContextEventProcessorRegistry;
import org.eclipse.sirius.components.core.api.IIdentityService;
import org.eclipse.sirius.components.core.api.IObjectSearchService;
import org.eclipse.sirius.components.graphql.api.IExceptionWrapper;
import org.eclipse.sirius.components.graphql.api.IEditingContextDispatcher;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.syson.export.ElementSerializer;
import org.openmbee.opensysml.syson.export.ProjectTextExporter;
import org.openmbee.opensysml.syson.menu.OpenSysMLTreeItemPaletteCustomizer;
import org.openmbee.opensysml.syson.run.RunResultStore;
import org.openmbee.opensysml.syson.run.RunWithOpenSysMLService;
import org.openmbee.opensysml.syson.validation.OpenSysMLValidationService;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;

class OpenSysMLAutoConfigurationTest {
    @Test
    void registersEachPluginBeanOnce() {
        new ApplicationContextRunner()
                .withUserConfiguration(OpenSysMLAutoConfiguration.class)
                .withBean(Connection.class, () -> mock(Connection.class))
                .withBean(IIdentityService.class, () -> mock(IIdentityService.class))
                .withBean(IObjectSearchService.class, () -> mock(IObjectSearchService.class))
                .withBean(IEditingContextEventProcessorRegistry.class,
                        () -> mock(IEditingContextEventProcessorRegistry.class))
                .withBean(IExceptionWrapper.class, () -> mock(IExceptionWrapper.class))
                .withBean(IEditingContextDispatcher.class, () -> mock(IEditingContextDispatcher.class))
                .withBean(tools.jackson.databind.ObjectMapper.class, () -> mock(tools.jackson.databind.ObjectMapper.class))
                .run(context -> {
                    assertThat(context.getBeansOfType(ElementSerializer.class)).hasSize(1);
                    assertThat(context.getBeansOfType(RunResultStore.class)).hasSize(1);
                    assertThat(context.getBeansOfType(ProjectTextExporter.class)).hasSize(1);
                    assertThat(context.getBeansOfType(RunWithOpenSysMLService.class)).hasSize(1);
                    assertThat(context.getBeansOfType(OpenSysMLTreeItemPaletteCustomizer.class)).hasSize(1);
                    assertThat(context.getBeansOfType(OpenSysMLValidationService.class)).hasSize(1);
                });
    }
}
