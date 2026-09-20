package org.openmbee.opensysml.syson;

import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.syson.export.ElementSerializer;
import org.openmbee.opensysml.syson.export.ProjectTextExporter;
import org.openmbee.opensysml.syson.export.SysONElementSerializer;
import org.openmbee.opensysml.syson.run.RunResultStore;
import org.eclipse.sirius.components.core.api.IIdentityService;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.ComponentScan;

@AutoConfiguration
@EnableConfigurationProperties(OpenSysMLProperties.class)
@ComponentScan(basePackageClasses = OpenSysMLAutoConfiguration.class)
public class OpenSysMLAutoConfiguration {
    @Bean(destroyMethod = "close")
    public Connection openSysMLConnection(OpenSysMLProperties properties) {
        return Connection.open(properties.toConnectionOptions());
    }

    @Bean
    public ElementSerializer elementSerializer() {
        return new SysONElementSerializer();
    }

    @Bean
    public ProjectTextExporter projectTextExporter(ElementSerializer serializer, IIdentityService identityService) {
        return new ProjectTextExporter(serializer, identityService);
    }

    @Bean
    public RunResultStore runResultStore() {
        return new RunResultStore();
    }
}
